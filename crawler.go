// gocrawl is a polite, slim and concurrent web crawler written in Go.
package gocrawl

import (
	"github.com/PuerkitoBio/purell"
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// Communication from worker to the master crawler, about the crawling of a URL
type workerResponse struct {
	host          string
	visited       bool
	sourceUrl     *url.URL
	harvestedUrls []*url.URL
	idleDeath     bool
}

// Communication from crawler to worker, about the URL to request
type workerCommand struct {
	u    *url.URL
	head bool
}

// EnqueueOrigin indicates to the crawler and the Filter extender function
// the origin of this URL.
type EnqueueOrigin int

const (
	EoSeed        EnqueueOrigin = iota // Seed URLs have this source
	EoHarvest                          // URLs harvested from a visit to a page have this source
	EoRedirect                         // URLs enqueued from a fetch redirection have this source by default
	EoError                            // URLs enqueued after an error
	EoCustomStart                      // Custom EnqueueOrigins should start at this value instead of iota
)

// Communication from extender to crawler about an URL to enqueue
type CrawlerCommand struct {
	URL    *url.URL
	Origin EnqueueOrigin
}

// The crawler itself, the master of the whole process
type Crawler struct {
	Options *Options

	// Internal fields
	logFunc   func(LogFlags, string, ...interface{})
	push      chan *workerResponse
	enqueue   chan *CrawlerCommand
	wg        *sync.WaitGroup
	endReason EndReason

	// keep visited URLs in map, O(1) access time vs O(n) for slice. The byte value
	// is of no use, but this is the smallest type possible.
	visited         map[string]byte // TODO : Keep a count of visits instead of useless byte?
	pushPopRefCount int
	visits          int
	workers         map[string]*worker
	hosts           []string
}

// Crawler constructor with a pre-initialized Options object.
func NewCrawlerWithOptions(opts *Options) *Crawler {
	ret := new(Crawler)
	ret.Options = opts
	return ret
}

// Crawler constructor with the specified extender object.
func NewCrawler(ext Extender) *Crawler {
	return NewCrawlerWithOptions(NewOptions(ext))
}

// Run starts the crawling process, based on the given seeds and the current
// Options settings. Execution stops either when MaxVisits is reached (if specified)
// or when no more URLs need visiting.
func (this *Crawler) Run(seeds ...string) EndReason {
	seeds = this.Options.Extender.Start(seeds)
	parsedSeeds := this.init(seeds)

	// Start with the seeds, and loop till death
	this.enqueueUrls(parsedSeeds, nil, EoSeed)
	this.collectUrls()

	this.Options.Extender.End(this.endReason)
	return this.endReason
}

// Initialize the Crawler's internal fields before a crawling execution.
func (this *Crawler) init(seeds []string) []*url.URL {
	// Helper log function, takes care of filtering based on level
	this.logFunc = getLogFunc(this.Options.Extender, this.Options.LogFlags, -1)

	// Parse the seeds and initialize the internal hosts slice
	parsedSeeds := this.parseSeeds(seeds)
	hostCount := len(this.hosts)
	l := len(parsedSeeds)
	this.logFunc(LogTrace, "init() - seeds length: %d", l)
	this.logFunc(LogTrace, "init() - host count: %d", hostCount)
	this.logFunc(LogInfo, "robot user-agent: %s", this.Options.RobotUserAgent)

	// Create a shiny new WaitGroup
	this.wg = new(sync.WaitGroup)

	// Initialize the visits fields
	this.visited = make(map[string]byte, l)
	this.pushPopRefCount, this.visits = 0, 0

	// Create the workers map and the push channel (the channel used by workers
	// to communicate back to the crawler)
	if this.Options.SameHostOnly {
		this.workers, this.push = make(map[string]*worker, hostCount),
			make(chan *workerResponse, hostCount)
	} else {
		this.workers, this.push = make(map[string]*worker, this.Options.HostBufferFactor*hostCount),
			make(chan *workerResponse, this.Options.HostBufferFactor*hostCount)
	}
	// Create and pass the enqueue channel
	this.enqueue = make(chan *CrawlerCommand, this.Options.EnqueueChanBuffer)
	this.setExtenderEnqueueChan()

	return parsedSeeds
}

// Set the Enqueue channel on the extender, based on the naming convention.
func (this *Crawler) setExtenderEnqueueChan() {
	defer func() {
		if err := recover(); err != nil {
			// Panic can happen if the field exists on a pointer struct, but that
			// pointer is nil.
			this.logFunc(LogError, "cannot set the enqueue channel: %s", err)
		}
	}()

	// Using reflection, check if the extender has a `EnqueueChan` field
	// of type `chan<- *CrawlerCommand`. If it does, set it to the crawler's
	// enqueue channel.
	v := reflect.ValueOf(this.Options.Extender)
	el := v.Elem()
	if el.Kind() != reflect.Struct {
		this.logFunc(LogInfo, "extender is not a struct, cannot set the enqueue channel")
		return
	}
	ec := el.FieldByName("EnqueueChan")
	if !ec.IsValid() {
		this.logFunc(LogInfo, "extender.EnqueueChan does not exist, cannot set the enqueue channel")
		return
	}
	t := ec.Type()
	if t.Kind() != reflect.Chan || t.ChanDir() != reflect.SendDir {
		this.logFunc(LogInfo, "extender.EnqueueChan is not of type chan<-*gocrawl.CrawlerCommand, cannot set the enqueue channel")
		return
	}
	tt := t.Elem()
	if tt.Kind() != reflect.Ptr {
		this.logFunc(LogInfo, "extender.EnqueueChan is not of type chan<-*gocrawl.CrawlerCommand, cannot set the enqueue channel")
		return
	}
	ttt := tt.Elem()
	if ttt.Kind() != reflect.Struct || ttt.Name() != "CrawlerCommand" {
		this.logFunc(LogInfo, "extender.EnqueueChan is not of type chan<-*gocrawl.CrawlerCommand, cannot set the enqueue channel")
		return
	}

	src := reflect.ValueOf(this.enqueue)
	ec.Set(src)
}

// Parse the seeds URL strings to URL objects, and return the URL objects slice
func (this *Crawler) parseSeeds(seeds []string) []*url.URL {
	// Translate seeds strings to URLs, normalized right away (to allow host count)
	this.hosts = make([]string, 0, len(seeds))
	parsedSeeds := make([]*url.URL, 0, len(seeds))

	for _, s := range seeds {
		// Very deliberately ignore error. If the string URL is not parseable, the normalization
		// will fail, so we will get the error (and propagate it) there. The parsed original
		// URL will only be returned if normalization succeeded.
		oriU, _ := url.Parse(s)

		// Normalize the URL, so that the host count is based on normalized data.
		if u, e := purell.NormalizeURLString(s, this.Options.URLNormalizationFlags); e != nil {
			this.Options.Extender.Error(newCrawlError(e, CekParseSeed, nil))
			this.logFunc(LogError, "ERROR parsing seed %s", s)
		} else {
			// Parse into a URL object the normalized URL (to use its .Host field)
			if parsed, e := url.Parse(u); e != nil {
				this.Options.Extender.Error(newCrawlError(e, CekParseNormalizedSeed, nil))
				this.logFunc(LogError, "ERROR parsing normalized seed %s", u)
			} else {
				// Add the ORIGINAL parsed URL to the parsedSeeds slice (if the normalized URL
				// could be parsed, the original could too).
				parsedSeeds = append(parsedSeeds, oriU)
				// Add this normalized URL's host if it is not already there.
				if indexInStrings(this.hosts, parsed.Host) == -1 {
					this.hosts = append(this.hosts, parsed.Host)
				}
			}
		}
	}

	return parsedSeeds
}

// Launch a new worker goroutine for a given host.
func (this *Crawler) launchWorker(u *url.URL) *worker {
	// Initialize index and channels
	i := len(this.workers) + 1
	pop, stop := newPopChannel(), make(chan bool, 1)

	// Create the worker
	w := &worker{
		u.Host,
		i,
		this.Options.UserAgent,
		this.Options.RobotUserAgent,
		this.push,
		pop,
		stop,
		this.wg,
		nil,
		this.enqueue,
		this.Options.CrawlDelay,
		this.Options.WorkerIdleTTL,
		nil,
		this.Options.Extender,
		getLogFunc(this.Options.Extender, this.Options.LogFlags, i),
		nil,
		0}

	// Increment wait group count
	this.wg.Add(1)

	// Launch worker
	go w.run()
	this.logFunc(LogInfo, "worker %d launched for host %s", i, w.host)
	this.workers[w.host] = w

	return w
}

// Check if the specified URL is from the same host as its source URL, or if
// nil, from the same host as one of the seed URLs.
func (this *Crawler) isSameHost(u *url.URL, sourceUrl *url.URL) bool {
	// If there is a source URL, then just check if the new URL is from the same host
	if sourceUrl != nil {
		return u.Host == sourceUrl.Host
	}

	// Otherwise, check if the URL is from one of the seed hosts
	return indexInStrings(this.hosts, u.Host) != -1
}

// Enqueue the URLs returned from the worker, as long as it complies with the
// selection policies.
func (this *Crawler) enqueueUrls(harvestedUrls []*url.URL, sourceUrl *url.URL, origin EnqueueOrigin) (cnt int) {
	for _, u := range harvestedUrls {
		var isVisited, enqueue, head bool
		var hr HeadRequestMode

		// Create a copy of the original url
		var rawU url.URL
		rawU = *u

		// Normalize URL
		purell.NormalizeURL(u, this.Options.URLNormalizationFlags)
		// Cannot directly enqueue a robots.txt URL, since it is managed as a special case
		// in the worker (doesn't return a response to crawler).
		if isRobotsTxtUrl(u) {
			continue
		}
		// Check if it has been visited before, using the normalized URL
		_, isVisited = this.visited[u.String()]

		// Filter the URL - TODO : Priority is ignored at the moment
		// The normalized URL is used for Filter
		if enqueue, _, hr = this.Options.Extender.Filter(u, sourceUrl, isVisited, origin); !enqueue {
			// Filter said NOT to use this url, so continue with next
			this.logFunc(LogIgnored, "ignore on filter policy: %s", u.String())
			continue
		}

		// Even if filter said to use the URL, it still MUST be absolute, http(s)-prefixed,
		// and comply with the same host policy if requested. Use normalized URL.
		if !u.IsAbs() {
			// Only absolute URLs are processed, so ignore
			this.logFunc(LogIgnored, "ignore on absolute policy: %s", u.String())

		} else if !strings.HasPrefix(u.Scheme, "http") { // Again, normalized URL
			this.logFunc(LogIgnored, "ignore on scheme policy: %s", u.String())

		} else if this.Options.SameHostOnly && !this.isSameHost(u, sourceUrl) { // Again, normalized URL
			// Only allow URLs coming from the same host
			this.logFunc(LogIgnored, "ignore on same host policy: %s", u.String())

		} else {
			// All is good, visit this URL (robots.txt verification is done by worker)

			// Possible caveat: if the normalization changes the host, it is possible
			// that the robots.txt fetched for this host would differ from the one for
			// the unnormalized host. However, this should be rare, and is a weird
			// behaviour from the host (i.e. why would site.com differ in its rules
			// from www.site.com) and can be fixed by using a different normalization
			// flag. So this is an acceptable behaviour for gocrawl.

			// Launch worker if required, based on the host of the normalized URL
			w, ok := this.workers[u.Host]
			if !ok {
				// No worker exists for this host, launch a new one
				w = this.launchWorker(u)
				// Automatically enqueue the robots.txt URL as first in line
				if robUrl, e := getRobotsTxtUrl(u); e != nil {
					this.Options.Extender.Error(newCrawlError(e, CekParseRobots, u))
					this.logFunc(LogError, "ERROR parsing robots.txt from %s: %s", u.String(), e.Error())
				} else {
					this.logFunc(LogEnqueued, "enqueue: %s", robUrl.String())
					this.Options.Extender.Enqueued(robUrl, sourceUrl)
					w.pop.stack(&workerCommand{robUrl, false})
				}
			}

			cnt++
			this.logFunc(LogEnqueued, "enqueue: %s", rawU.String())
			this.Options.Extender.Enqueued(&rawU, sourceUrl)
			switch hr {
			case HrmIgnore:
				head = false
			case HrmRequest:
				head = true
			default:
				head = this.Options.HeadBeforeGet
			}
			// The non-normalized URL is enqueued.
			w.pop.stack(&workerCommand{&rawU, head})
			this.pushPopRefCount++

			// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
			// (unless denied by robots.txt, but this is out of our hands, for all we
			// care, it is visited).
			if !isVisited {
				// The visited map works with the normalized URL
				this.visited[u.String()] = '0'
			}
		}
	}
	return
}

// This is the main loop of the crawler, waiting for responses from the workers
// and processing these responses.
func (this *Crawler) collectUrls() {
	defer func() {
		this.logFunc(LogInfo, "waiting for goroutines to complete...")
		this.wg.Wait()
		this.logFunc(LogInfo, "crawler done.")
	}()

	stopAll := func() {
		this.logFunc(LogInfo, "sending STOP signals...")
		for _, w := range this.workers {
			w.stop <- true
		}
	}

	for {
		select {
		case res := <-this.push:
			// Received a response, check if it contains URLs to enqueue
			if res.visited {
				this.visits++
				if this.Options.MaxVisits > 0 && this.visits >= this.Options.MaxVisits {
					// Limit reached, request workers to stop
					this.endReason = ErMaxVisits
					stopAll()
					return
				}
			}
			if res.idleDeath {
				// The worker timed out from its Idle TTL delay, remove from active workers
				delete(this.workers, res.host)
				this.logFunc(LogInfo, "worker for host %s cleared on idle policy", res.host)
			} else {
				this.enqueueUrls(res.harvestedUrls, res.sourceUrl, EoHarvest)
				this.pushPopRefCount--
			}

		case enq := <-this.enqueue:
			// Received a command to enqueue a URL, proceed
			this.logFunc(LogTrace, "receive url %s", enq.URL.String())
			this.enqueueUrls([]*url.URL{enq.URL}, nil, enq.Origin)

		default:
			// Check if refcount is zero
			if this.pushPopRefCount == 0 {
				this.endReason = ErDone
				stopAll()
				return
			} else {
				runtime.Gosched()
			}
		}
	}
}
