package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	//"github.com/temoto/robotstxt.go"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
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
	Logger         *log.Logger
	CrawlDelay     time.Duration // Applied per host
	SameHostOnly   bool

	push            chan *urlContainer
	pop             popChannel
	visited         []string
	pushPopRefCount int
}

func NewCrawler(seeds ...string) *Crawler {
	// Use sane defaults
	ret := new(Crawler)
	ret.UserAgent = DefaultUserAgent
	ret.RobotUserAgent = DefaultRobotUserAgent
	ret.Logger = log.New(os.Stdout, "gocrawl ", log.LstdFlags|log.Lmicroseconds)
	ret.CrawlDelay = DefaultCrawlDelay
	ret.MaxGoroutines = 1
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
	// Initialize the channels

	// The pop channel will be stacked, so only a buffer of 1 is required
	// see http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels
	this.pop = newPopChannel()

	// The push channel needs a buffer equal to the # of goroutines (+1?)
	this.push = make(chan *urlContainer, this.MaxGoroutines)

	// Feed pop channel with seeds
	this.enqueueUrls(&urlContainer{nil, this.Seeds})

	a := &agent{cb, this.push, this.pop, this.Logger, 1}
	go a.Run()
	this.Logger.Println("Agent 1 launched.")

	// Start feeding
	this.feedUrls()
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
		if len(u.Scheme) == 0 || len(u.Host) == 0 {
			// Only absolute URLs are processed, so ignore
			this.Logger.Printf("Ignored URL on Absolute URL policy %s\n", u.String())

		} else if cont.sourceUrl != nil && u.Host != cont.sourceUrl.Host && this.SameHostOnly {
			// Only allow URLs coming from the same host
			this.Logger.Printf("Ignored URL on Same Host policy: %s\n", u.String())

		} else if !this.isVisited(u) {
			cnt++
			//this.Logger.Printf("Enqueue URL %s\n", u.String())
			this.pop.stack(u)
			this.pushPopRefCount++

			// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
			this.visited = append(this.visited, u.String())

		} else {
			this.Logger.Printf("Ignored URL on Already Visited policy: %s\n", u.String())
		}
	}
	return
}

func (this *Crawler) feedUrls() {
	for {
		select {
		case cont := <-this.push:
			// Received a URL container to enqueue
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
