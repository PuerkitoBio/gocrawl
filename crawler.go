package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	//"github.com/temoto/robotstxt.go"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type LogLevel int

const (
	LogError LogLevel = 1 << iota
	LogInfo
	LogTrace
	LogNone LogLevel = 0
)

const (
	DefaultUserAgent      string        = `Mozilla/5.0 (Windows NT 6.1; rv:15.0) Gecko/20120716 Firefox/15.0a2`
	DefaultRobotUserAgent string        = `gocrawl (Googlebot)`
	DefaultCrawlDelay     time.Duration = 5 * time.Second
)

type VisitorFunc func(*http.Response, *goquery.Document) ([]*url.URL, bool)

type urlContainer struct {
	sourceUrl     *url.URL
	harvestedUrls []*url.URL
}

type Crawler struct {
	Seeds          []*url.URL
	UserAgent      string
	RobotUserAgent string
	MaxVisits      int
	MaxGoroutines  int

	Logger   *log.Logger
	LogLevel LogLevel

	CrawlDelay   time.Duration // Applied per host
	SameHostOnly bool

	push            chan *urlContainer
	pop             popChannel
	visited         []string
	pushPopRefCount int
	visits          int
}

func NewCrawler(seeds ...string) *Crawler {
	// Use sane defaults
	ret := new(Crawler)
	ret.UserAgent = DefaultUserAgent
	ret.RobotUserAgent = DefaultRobotUserAgent
	ret.Logger = log.New(os.Stdout, "gocrawl ", log.LstdFlags|log.Lmicroseconds)
	ret.LogLevel = LogError
	ret.CrawlDelay = DefaultCrawlDelay
	ret.MaxGoroutines = 4
	ret.SameHostOnly = true

	// Translate seeds strings to URLs
	for _, s := range seeds {
		if u, e := url.Parse(s); e == nil {
			ret.Seeds = append(ret.Seeds, u)
		}
	}

	return ret
}

func (this *Crawler) Run(cb VisitorFunc) {
	// TODO : Check options before start

	// Initialize the channels

	// The pop channel will be stacked, so only a buffer of 1 is required
	// see http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels
	this.pop = newPopChannel()

	// The push channel needs a buffer equal to the # of goroutines (+1?)
	this.push = make(chan *urlContainer, this.MaxGoroutines)

	// Feed pop channel with seeds
	this.enqueueUrls(&urlContainer{nil, this.Seeds})

	for i := 1; i <= this.MaxGoroutines; i++ {
		a := &agent{cb, this.push, this.pop, this.UserAgent, this.Logger, this.LogLevel, i}
		go a.Run()
		if this.LogLevel|LogTrace == LogTrace {
			this.Logger.Printf("Agent %d launched.\n", i)
		}
	}

	// Start collecting results
	this.collectUrls()
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

		// TODO : Normalize URL

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

		} else if !this.isVisited(u) {
			cnt++
			if this.LogLevel|LogTrace == LogTrace {
				this.Logger.Printf("Enqueue URL %s\n", u.String())
			}
			this.pop.stack(u)
			this.pushPopRefCount++

			// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
			this.visited = append(this.visited, u.String())

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
				close(this.pop)
				return
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
