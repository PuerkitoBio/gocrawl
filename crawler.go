package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Default options
const (
	DefaultUserAgent          string                    = `Mozilla/5.0 (Windows NT 6.1; rv:15.0) Gecko/20120716 Firefox/15.0a2`
	DefaultRobotUserAgent     string                    = `gocrawl (Googlebot)`
	DefaultCrawlDelay         time.Duration             = 5 * time.Second
	DefaultNormalizationFlags purell.NormalizationFlags = purell.FlagsAllGreedy
)

// URL container returned by agents to the crawler
type urlContainer struct {
	sourceUrl     *url.URL
	harvestedUrls []*url.URL
}

type workerInfo struct {
	w   *worker
	pop popChannel
}

// The crawler itself, the master of the whole process
type Crawler struct {
	Seeds          []*url.URL
	UserAgent      string
	RobotUserAgent string
	MaxVisits      int
	UrlVisitor     func(*http.Response, *goquery.Document) ([]*url.URL, bool)

	Logger   *log.Logger
	LogLevel LogLevel
	logFunc  func(LogLevel, string, ...interface{})

	CrawlDelay            time.Duration // Applied per host
	SameHostOnly          bool
	UrlNormalizationFlags purell.NormalizationFlags
	UrlSelector           func(target *url.URL, origin *url.URL, isVisited bool) bool

	push chan *urlContainer
	stop chan bool
	wg   sync.WaitGroup

	visited          []string
	pushPopRefCount  int
	visits           int
	workers          map[string]*workerInfo
	initialHostCount int
}

// Major steps needing hooks (implement as middleware funcs?):
//
// - Prior to add to queue
//   * Normalize URL
//   * Is already visited?
//   * Is allowed by Robots.txt?
//   * Is same host
//   * Custom
//   * Is absolute URL
//   * Is http(s) scheme
//   * Is an interesting URL (based on pattern)

func New(visitor func(*http.Response, *goquery.Document) ([]*url.URL, bool), seeds ...string) *Crawler {

	// Use sane defaults
	ret := new(Crawler)
	ret.UserAgent = DefaultUserAgent
	ret.RobotUserAgent = DefaultRobotUserAgent
	ret.Logger = log.New(os.Stdout, "gocrawl ", log.LstdFlags|log.Lmicroseconds)
	ret.LogLevel = LogTrace
	ret.CrawlDelay = DefaultCrawlDelay
	ret.SameHostOnly = true
	ret.UrlNormalizationFlags = DefaultNormalizationFlags
	ret.UrlVisitor = visitor

	// Translate seeds strings to URLs, normalized right away (to allow host count)
	hosts := make([]string, 10)
	for _, s := range seeds {
		if u, e := purell.NormalizeURLString(s, ret.UrlNormalizationFlags); e != nil {
			if ret.LogLevel|LogError == LogError {
				ret.Logger.Printf("Error parsing seed URL %s\n", s)
			}
		} else {
			parsed, _ := url.Parse(u)
			ret.Seeds = append(ret.Seeds, parsed)
			if sort.SearchStrings(hosts, parsed.Host) < 0 {
				hosts = append(hosts, parsed.Host)
				sort.Strings(hosts)
			}
		}
	}
	ret.initialHostCount = len(hosts)

	return ret
}

// TODO : If run as a goroutine, can the caller modify fields on the struct? Provide IsRunning(), Stop(), Pause()?

func (this *Crawler) Run() {
	// TODO : Check options before start

	// The stop channel is used to tell agents to stop looping
	this.stop = make(chan bool)

	// Help log function, takes care of filtering based on level
	this.logFunc = getLogFunc(this.Logger, this.LogLevel, -1)

	// Create the workers map
	this.logFunc(LogTrace, "Initial host count is %d\n", this.initialHostCount)
	if this.SameHostOnly {
		this.workers = make(map[string]*workerInfo, this.initialHostCount)
		this.push = make(chan *urlContainer, this.initialHostCount)
	} else {
		this.workers = make(map[string]*workerInfo, 10*this.initialHostCount)
		this.push = make(chan *urlContainer, 10*this.initialHostCount)
	}

	// Start with the seeds, and loop till death
	this.enqueueUrls(&urlContainer{nil, this.Seeds})
	this.collectUrls()
}

func (this *Crawler) launchWorker(u *url.URL) *workerInfo {
	i := len(this.workers) + 1
	pop := newPopChannel()

	w := &worker{this.UrlVisitor,
		this.push,
		pop,
		this.stop,
		this.UserAgent,
		this.RobotUserAgent,
		getLogFunc(this.Logger, this.LogLevel, i),
		i,
		this.wg,
		this.CrawlDelay,
		nil}

	this.wg.Add(1)
	go w.Run()
	this.logFunc(LogTrace, "Worker %d launched.\n", i)
	this.workers[u.Host] = &workerInfo{w, pop}

	return this.workers[u.Host]
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
		if this.UrlSelector != nil {
			if forceEnqueue = this.UrlSelector(u, cont.sourceUrl, isVisited); !forceEnqueue {
				// Custom selector said NOT to use this url, so continue with next
				continue
			}
		}

		// Even if custom selector said to use the URL, it still MUST be absolute, http-prefixed,
		// and comply with the same host if requested.
		if len(u.Scheme) == 0 || len(u.Host) == 0 {
			// Only absolute URLs are processed, so ignore
			this.logFunc(LogTrace, "Ignored URL on Absolute URL policy %s\n", u.String())

		} else if !strings.HasPrefix(u.Scheme, "http") {
			this.logFunc(LogTrace, "Ignored URL on Invalid Scheme policy %s\n", u.String())

		} else if cont.sourceUrl != nil && u.Host != cont.sourceUrl.Host && this.SameHostOnly {
			// Only allow URLs coming from the same host
			this.logFunc(LogTrace, "Ignored URL on Same Host policy: %s\n", u.String())

		} else if !isVisited || forceEnqueue {
			// All is good, visit this URL (robots.txt verification is done by worker)

			// Launch worker if required
			wi, ok := this.workers[u.Host]
			if !ok {
				wi = this.launchWorker(u)
				// Automatically enqueue the robots.txt URL as first in line
				// TODO : Error
				robUrl, _ := u.Parse("/robots.txt")
				this.logFunc(LogTrace, "Enqueue URL %s\n", robUrl.String())
				wi.pop.stack(robUrl)
			}

			cnt++
			this.logFunc(LogTrace, "Enqueue URL %s\n", u.String())
			wi.pop.stack(u)
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
	for {
		select {
		case cont := <-this.push:
			// Received a URL container to enqueue
			this.visits++
			if this.visits >= this.MaxVisits {
				this.stop <- true
				return
			}
			this.enqueueUrls(cont)
			this.pushPopRefCount--

		default:
			// Check if refcount is zero
			if this.pushPopRefCount == 0 {
				this.stop <- true
				return
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
