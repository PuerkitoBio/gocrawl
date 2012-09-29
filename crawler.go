package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// URL container returned by workers to the crawler
type urlContainer struct {
	sourceUrl     *url.URL
	visited       bool
	harvestedUrls []*url.URL
}

// The crawler itself, the master of the whole process
type Crawler struct {
	Options *Options

	// Internal fields
	logFunc func(LogFlags, string, ...interface{})
	push    chan *urlContainer
	wg      sync.WaitGroup

	visited         []string
	pushPopRefCount int
	visits          int
	workers         map[string]*worker
}

func NewCrawlerWithOptions(opts *Options) *Crawler {
	ret := new(Crawler)
	ret.Options = opts
	return ret
}

func NewCrawler(visitor func(*http.Response, *goquery.Document) ([]*url.URL, bool),
	urlSelector func(*url.URL, *url.URL, bool) bool) *Crawler {
	return NewCrawlerWithOptions(NewOptions(visitor, urlSelector))
}

func (this *Crawler) Run(seeds ...string) {
	// Helper log function, takes care of filtering based on level
	this.logFunc = getLogFunc(this.Options.Logger, this.Options.LogFlags, -1)

	parsedSeeds, hostCount := this.parseSeeds(seeds)
	this.logFunc(LogTrace, "Parsed seeds length: %d\n", len(parsedSeeds))
	this.logFunc(LogTrace, "Initial host count is %d\n", hostCount)
	this.logFunc(LogTrace, "Robot user-agent is %s\n", this.Options.RobotUserAgent)

	// Create the workers map and the push channel (send harvested URLs to the crawler to enqueue)
	if this.Options.SameHostOnly {
		this.workers = make(map[string]*worker, hostCount)
		this.push = make(chan *urlContainer, hostCount)
	} else {
		this.workers = make(map[string]*worker, 10*hostCount)
		this.push = make(chan *urlContainer, 10*hostCount)
	}

	// Start with the seeds, and loop till death
	this.enqueueUrls(&urlContainer{nil, false, parsedSeeds})
	this.collectUrls()
}

// Parse the seeds URL strings to URL objects, and return the URL objects slice,
// along with the count of distinct hosts.
func (this *Crawler) parseSeeds(seeds []string) ([]*url.URL, int) {
	// Translate seeds strings to URLs, normalized right away (to allow host count)
	hosts := make([]string, 0, len(seeds))
	parsedSeeds := make([]*url.URL, 0, len(seeds))

	for _, s := range seeds {
		if u, e := purell.NormalizeURLString(s, this.Options.URLNormalizationFlags); e != nil {
			this.logFunc(LogError, "Error parsing seed URL %s\n", s)
		} else {
			if parsed, e := url.Parse(u); e != nil {
				this.logFunc(LogError, "Error parsing normalized seed URL %s\n", u)
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

func (this *Crawler) launchWorker(u *url.URL) *worker {
	// Initialize index and channels
	i := len(this.workers) + 1
	pop := newPopChannel()
	stop := make(chan bool, 1)

	// Create the worker
	w := &worker{this.Options.URLVisitor,
		this.push,
		pop,
		stop,
		this.Options.UserAgent,
		this.Options.RobotUserAgent,
		getLogFunc(this.Options.Logger, this.Options.LogFlags, i),
		i,
		&this.wg,
		this.Options.CrawlDelay,
		nil,
		this.Options.Fetcher}

	// Increment wait group count
	this.wg.Add(1)

	// Launch worker
	go w.run()
	this.logFunc(LogTrace, "Worker %d launched.\n", i)
	this.workers[u.Host] = w

	return w
}

func (this *Crawler) isVisited(u *url.URL) bool {
	for _, v := range this.visited {
		if u.String() == v {
			return true
		}
	}
	return false
}

func (this *Crawler) enqueueUrls(cont *urlContainer) (cnt int) {
	for _, u := range cont.harvestedUrls {
		var isVisited, forceEnqueue bool

		// Normalize URL
		purell.NormalizeURL(u, DefaultNormalizationFlags)
		isVisited = this.isVisited(u)

		// If a selector callback is specified, use this to filter URL
		if this.Options.URLSelector != nil {
			if forceEnqueue = this.Options.URLSelector(u, cont.sourceUrl, isVisited); !forceEnqueue {
				// Custom selector said NOT to use this url, so continue with next
				this.logFunc(LogTrace, "Ignore URL on Custom Selector policy %s\n", u.String())
				continue
			}
		}

		// Even if custom selector said to use the URL, it still MUST be absolute, http(s)-prefixed,
		// and comply with the same host policy if requested.
		if !u.IsAbs() {
			// Only absolute URLs are processed, so ignore
			this.logFunc(LogTrace, "Ignore URL on Absolute URL policy %s\n", u.String())

		} else if !strings.HasPrefix(u.Scheme, "http") {
			this.logFunc(LogTrace, "Ignore URL on Invalid Scheme policy %s\n", u.String())

		} else if cont.sourceUrl != nil && u.Host != cont.sourceUrl.Host && this.Options.SameHostOnly {
			// Only allow URLs coming from the same host
			this.logFunc(LogTrace, "Ignore URL on Same Host policy: %s\n", u.String())

		} else if !isVisited || forceEnqueue {
			// All is good, visit this URL (robots.txt verification is done by worker)

			// Launch worker if required
			w, ok := this.workers[u.Host]
			if !ok {
				w = this.launchWorker(u)
				// Automatically enqueue the robots.txt URL as first in line
				if robUrl, e := getRobotsTxtUrl(u); e != nil {
					this.logFunc(LogError, "Error parsing robots.txt URL from %s: %s\n", u.String(), e.Error())
				} else {
					this.logFunc(LogTrace, "Enqueue URL %s\n", robUrl.String())
					w.pop.stack(robUrl)
				}
			}

			cnt++
			this.logFunc(LogTrace, "Enqueue URL %s\n", u.String())
			w.pop.stack(u)
			this.pushPopRefCount++

			// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
			if !isVisited {
				this.visited = append(this.visited, u.String())
			}

		} else {
			this.logFunc(LogTrace, "Ignored URL on Already Visited policy: %s\n", u.String())
		}
	}
	return
}

func (this *Crawler) collectUrls() {
	defer func() {
		this.logFunc(LogTrace, "Waiting for goroutines to complete...\n")
		this.wg.Wait()
		this.logFunc(LogTrace, "Done.\n")
	}()

	stopAll := func() {
		this.logFunc(LogTrace, "Sending STOP signals...\n")
		for _, w := range this.workers {
			w.stop <- true
		}
	}

	for {
		select {
		case cont := <-this.push:
			// Received a URL container to enqueue
			if cont.visited {
				this.visits++
				if this.Options.MaxVisits > 0 && this.visits >= this.Options.MaxVisits {
					// Limit reached, request workers to stop
					stopAll()
					return
				}
			}
			this.enqueueUrls(cont)
			this.pushPopRefCount--

		case <-time.After(100 * time.Millisecond):
			// Check if refcount is zero
			if this.pushPopRefCount == 0 {
				stopAll()
				return
			}
		}
	}
}
