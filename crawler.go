package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type LogLevel uint

// Log levels for the library's logger
const (
	LogError LogLevel = 1 << iota
	LogInfo
	LogTrace
	LogNone LogLevel = 0
)

// Default options
const (
	DefaultUserAgent          string                    = `Mozilla/5.0 (Windows NT 6.1; rv:15.0) Gecko/20120716 Firefox/15.0a2`
	DefaultRobotUserAgent     string                    = `gocrawl (Googlebot)`
	DefaultCrawlDelay         time.Duration             = 5 * time.Second
	DefaultNormalizationFlags purell.NormalizationFlags = purell.FlagsUnsafe | purell.FlagDecodeDWORDHost |
		purell.FlagDecodeOctalHost | purell.FlagDecodeHexHost | purell.FlagRemoveUnnecessaryHostDots |
		purell.FlagRemoveEmptyPortSeparator
)

// URL container returned by agents to the crawler
type urlContainer struct {
	sourceUrl     *url.URL
	harvestedUrls []*url.URL
}

// The crawler itself, the master of the whole process
type Crawler struct {
	Seeds          []*url.URL
	UserAgent      string
	RobotUserAgent string
	MaxVisits      int
	MaxGoroutines  int
	UrlVisitor     func(*http.Response, *goquery.Document) ([]*url.URL, bool)

	Logger   *log.Logger
	LogLevel LogLevel

	CrawlDelay            time.Duration // Applied per host
	SameHostOnly          bool
	UrlNormalizationFlags purell.NormalizationFlags
	UrlSelector           func(target *url.URL, origin *url.URL, isVisited bool) bool

	push chan *urlContainer
	pop  popChannel
	stop chan bool

	visited         []string
	pushPopRefCount int
	visits          int
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
	ret.MaxGoroutines = 4
	ret.SameHostOnly = true
	ret.UrlNormalizationFlags = DefaultNormalizationFlags
	ret.UrlVisitor = visitor

	// Translate seeds strings to URLs
	for _, s := range seeds {
		if u, e := url.Parse(s); e == nil {
			ret.Seeds = append(ret.Seeds, u)
		}
	}

	return ret
}

func (this *Crawler) Run() {
	// TODO : Check options before start

	// The pop channel will be stacked, so only a buffer of 1 is required
	// see http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels
	this.pop = newPopChannel()

	// The push channel needs a buffer equal to the # of goroutines (+1?)
	this.push = make(chan *urlContainer, this.MaxGoroutines)

	// The stop channel is used to tell agents to stop looping
	this.stop = make(chan bool)

	this.enqueueUrls(&urlContainer{nil, this.Seeds})
	this.launchAgents()
	this.collectUrls()
}

func (this *Crawler) launchAgents() {
	for i := 1; i <= this.MaxGoroutines; i++ {
		a := &agent{this.UrlVisitor, this.push, this.pop, this.stop, this.UserAgent, this.Logger, this.LogLevel, i}
		go a.Run()
		if this.LogLevel|LogTrace == LogTrace {
			this.Logger.Printf("Agent %d launched.\n", i)
		}
	}
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
			if this.LogLevel|LogTrace == LogTrace {
				this.Logger.Printf("Ignored URL on Absolute URL policy %s\n", u.String())
			}

		} else if !strings.HasPrefix(u.Scheme, "http") {
			if this.LogLevel|LogTrace == LogTrace {
				this.Logger.Printf("Ignored URL on Invalid Scheme policy %s\n", u.String())
			}

		} else if cont.sourceUrl != nil && u.Host != cont.sourceUrl.Host && this.SameHostOnly {
			// Only allow URLs coming from the same host
			if this.LogLevel|LogTrace == LogTrace {
				this.Logger.Printf("Ignored URL on Same Host policy: %s\n", u.String())
			}

		} else if !isVisited || forceEnqueue {
			// So this URL is targeted to be visited, check robots.txt rule as last
			// check (since it is costly, and it's the last barrier to visit)
			if ok, e := isUrlAllowedPerRobots(u, this.RobotUserAgent); !ok || e != nil {
				if e != nil && this.LogLevel|LogError == LogError {
					this.Logger.Printf("Error querying robots.txt for URL %s: %s\n", u.String(), e.Error())
				} else if !ok && this.LogLevel|LogTrace == LogTrace {
					this.Logger.Printf("Ignored URL on Robots.txt policy %s\n", u.String())
				}

			} else {
				// All is good, visit this URL
				cnt++
				if this.LogLevel|LogTrace == LogTrace {
					this.Logger.Printf("Enqueue URL %s\n", u.String())
				}
				this.pop.stack(u)
				this.pushPopRefCount++

				// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
				if !isVisited {
					this.visited = append(this.visited, u.String())
				}
			}

		} else {
			if this.LogLevel|LogTrace == LogTrace {
				this.Logger.Printf("Ignored URL on Already Visited policy: %s\n", u.String())
			}
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
				close(this.pop)
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
