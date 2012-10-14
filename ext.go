package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"time"
)

type EndReason uint8
type CrawlStep uint8

const (
	ErDone EndReason = iota
	ErMaxVisits
	ErError
)

const (
	CsStart CrawlStep = iota
	CsEnqueue
	CsFetch
	//...
)

type Extender interface {
	// TODO:  Panics: terminates crawling for this host?

	Start(seeds []string) []string
	End(reason EndReason)
	Error(err error, step CrawlStep)
	Enqueued(u *url.URL, from *url.URL)
	Visited(u *url.URL, harvested []*url.URL)

	Visit(*http.Response, *goquery.Document) (harvested []*url.URL, findLinks bool)
	Filter(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int)
	ComputeDelay(host string, robotsDelay time.Duration, lastFetch time.Duration) time.Duration
	Fetch(u *url.URL, userAgent string) (res *http.Response, err error) // TODO : Time the fetch for compute delay
	RequestRobots(u *url.URL) (request bool, data []byte)
}
