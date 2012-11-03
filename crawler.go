// gocrawl is a polite, slim and concurrent web crawler written in Go.
package gocrawl

import (
	"github.com/PuerkitoBio/purell"
	"net/url"
	"runtime"
	"strings"
	"sync"
)

// Communication from worker to the master crawler
type workerResponse struct {
	host    string
	visited bool

	sourceUrl     *url.URL
	harvestedUrls []*url.URL

	idleDeath bool
}

// Communication from crawler to worker, about the URL to request
type workerCommand struct {
	u    *url.URL
	head bool
}

// The crawler itself, the master of the whole process
type Crawler struct {
	Options *Options

	// Internal fields
	logFunc   func(LogFlags, string, ...interface{})
	push      chan *workerResponse
	wg        *sync.WaitGroup
	endReason EndReason

	// keep visited URLs in map, O(1) access time vs O(n) for slice. The byte value
	// is of no use, but this is the smallest type possible.
	visited         map[string]byte
	pushPopRefCount int
	visits          int
	workers         map[string]*worker
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
func (this *Crawler) Run(seeds ...string) {
	seeds = this.Options.Extender.Start(seeds)
	parsedSeeds := this.init(seeds)

	// Start with the seeds, and loop till death
	this.enqueueUrls(&workerResponse{"", false, nil, parsedSeeds, false})
	this.collectUrls()

	this.Options.Extender.End(this.endReason)
}

// Initialize the Crawler's internal fields before a crawling execution.
func (this *Crawler) init(seeds []string) []*url.URL {
	// Helper log function, takes care of filtering based on level
	this.logFunc = getLogFunc(this.Options.Extender, this.Options.LogFlags, -1)

	// Parse the seeds and get the host count
	parsedSeeds, hostCount := this.parseSeeds(seeds)
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
		this.workers, this.push = make(map[string]*worker, hostCount), make(chan *workerResponse, hostCount)
	} else {
		this.workers, this.push = make(map[string]*worker, 10*hostCount), make(chan *workerResponse, 10*hostCount)
	}

	return parsedSeeds
}

// Parse the seeds URL strings to URL objects, and return the URL objects slice,
// along with the count of distinct hosts.
func (this *Crawler) parseSeeds(seeds []string) ([]*url.URL, int) {
	// Translate seeds strings to URLs, normalized right away (to allow host count)
	hosts := make([]string, 0, len(seeds))
	parsedSeeds := make([]*url.URL, 0, len(seeds))

	for _, s := range seeds {
		if u, e := purell.NormalizeURLString(s, this.Options.URLNormalizationFlags); e != nil {
			this.Options.Extender.Error(newCrawlError(e, CekParseSeed, nil))
			this.logFunc(LogError, "ERROR parsing seed %s", s)
		} else {
			if parsed, e := url.Parse(u); e != nil {
				this.Options.Extender.Error(newCrawlError(e, CekParseNormalizedSeed, nil))
				this.logFunc(LogError, "ERROR parsing normalized seed %s", u)
			} else {
				parsedSeeds = append(parsedSeeds, parsed)
				if indexInStrings(hosts, parsed.Host) == -1 {
					hosts = append(hosts, parsed.Host)
				}
			}
		}
	}

	return parsedSeeds, len(hosts)
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

// Enqueue the URLs returned from the worker, as long as it complies with the
// selection policies.
func (this *Crawler) enqueueUrls(res *workerResponse) (cnt int) {
	for _, u := range res.harvestedUrls {
		var isVisited, enqueue, head bool
		var hr HeadRequestMode

		// Normalize URL
		purell.NormalizeURL(u, DefaultNormalizationFlags)
		_, isVisited = this.visited[u.String()]

		// Filter the URL - TODO : Priority is ignored at the moment
		if enqueue, _, hr = this.Options.Extender.Filter(u, res.sourceUrl, isVisited); !enqueue {
			// Filter said NOT to use this url, so continue with next
			this.logFunc(LogIgnored, "ignore on filter policy: %s", u.String())
			continue
		}

		// Even if filter said to use the URL, it still MUST be absolute, http(s)-prefixed,
		// and comply with the same host policy if requested.
		if !u.IsAbs() {
			// Only absolute URLs are processed, so ignore
			this.logFunc(LogIgnored, "ignore on absolute policy: %s", u.String())

		} else if !strings.HasPrefix(u.Scheme, "http") {
			this.logFunc(LogIgnored, "ignore on scheme policy: %s", u.String())

		} else if res.sourceUrl != nil && u.Host != res.sourceUrl.Host && this.Options.SameHostOnly {
			// Only allow URLs coming from the same host
			this.logFunc(LogIgnored, "ignore on same host policy: %s", u.String())

		} else {
			// All is good, visit this URL (robots.txt verification is done by worker)

			// Launch worker if required
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
					this.Options.Extender.Enqueued(robUrl, res.sourceUrl)
					w.pop.stack(&workerCommand{robUrl, false})
				}
			}

			cnt++
			this.logFunc(LogEnqueued, "enqueue: %s", u.String())
			this.Options.Extender.Enqueued(u, res.sourceUrl)
			switch hr {
			case HrmIgnore:
				head = false
			case HrmRequest:
				head = true
			default:
				head = this.Options.HeadBeforeGet
			}
			w.pop.stack(&workerCommand{u, head})
			this.pushPopRefCount++

			// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
			// (unless denied by robots.txt, but this is out of our hands, for all we
			// care, it is visited).
			if !isVisited {
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
				this.enqueueUrls(res)
				this.pushPopRefCount--
			}

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
