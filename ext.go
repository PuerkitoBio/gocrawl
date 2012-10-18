package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"time"
)

type EndReason uint8
type CrawlErrorKind uint8

const (
	ErDone EndReason = iota
	ErMaxVisits
	ErError
)

const (
	CekFetch CrawlErrorKind = iota
	CekParseRobots
	CekHttpStatusCode
	CekReadBody
	CekParseBody
	CekParseSeed
	CekParseNormalizedSeed
	CekProcessLinks
)

var defaultClient = new(http.Client)

type CrawlError struct {
	SourceError error
	ErrorKind   CrawlErrorKind
	URL         *url.URL
	msg         string
}

func (this CrawlError) Error() string {
	if this.SourceError != nil {
		return this.SourceError.Error()
	}
	return this.msg
}

func newCrawlError(e error, kind CrawlErrorKind, u *url.URL) error {
	return &CrawlError{e, kind, u, ""}
}

func newCrawlErrorMessage(msg string, kind CrawlErrorKind, u *url.URL) error {
	return &CrawlError{nil, kind, u, msg}
}

type Extender interface {
	Start(seeds []string) []string
	End(reason EndReason)
	Error(err error)

	ComputeDelay(host string, optsDelay time.Duration, robotsDelay time.Duration, lastFetch time.Duration) time.Duration
	Fetch(u *url.URL, userAgent string) (res *http.Response, err error)
	RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte)

	Filter(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int)
	Enqueued(u *url.URL, from *url.URL)
	Visit(*http.Response, *goquery.Document) (harvested []*url.URL, findLinks bool)
	Visited(u *url.URL, harvested []*url.URL)
	Disallowed(u *url.URL)
}

type DefaultExtender struct {
	visitor func(*http.Response, *goquery.Document) (harvested []*url.URL, findLinks bool)
	filter  func(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int)
}

// Return the same seeds as those received (those that were passed
// to Run() initially).
func (this *DefaultExtender) Start(seeds []string) []string {
	return seeds
}

// End is a no-op.
func (this *DefaultExtender) End(reason EndReason) {}

// Error is a no-op (logging is done automatically, regardless of the implementation
// of the Error() hook).
func (this *DefaultExtender) Error(err error) {}

// ComputeDelay returns the delay specified in the Crawler's Options, unless a
// crawl-delay is specified in the robots.txt file, which has precedence.
func (this *DefaultExtender) ComputeDelay(host string, optsDelay time.Duration,
	robotsDelay time.Duration, lastFetch time.Duration) time.Duration {
	if robotsDelay > 0 {
		return robotsDelay
	}
	return optsDelay
}

// Fetch requests the specified URL using the given user agent string. It uses
// Go's default http Client instance.
func (this *DefaultExtender) Fetch(u *url.URL, userAgent string) (res *http.Response, err error) {
	// Prepare the request with the right user agent
	req, e := http.NewRequest("GET", u.String(), nil)
	if e != nil {
		return nil, e
	}
	req.Header["User-Agent"] = []string{userAgent}
	return defaultClient.Do(req)
}

// Ask the worker to actually request (fetch) the Robots.txt document.
func (this *DefaultExtender) RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte) {
	return true, nil
}

// Enqueue the URL if it hasn't been visited yet.
func (this *DefaultExtender) Filter(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int) {
	if this.filter != nil {
		return this.filter(u, from, isVisited)
	}
	return !isVisited, 0
}

// Enqueued is a no-op.
func (this *DefaultExtender) Enqueued(u *url.URL, from *url.URL) {}

// Ask the worker to harvest the links in this page.
func (this *DefaultExtender) Visit(res *http.Response, doc *goquery.Document) (harvested []*url.URL, findLinks bool) {
	if this.visitor != nil {
		return this.visitor(res, doc)
	}
	return nil, true
}

// Visited is a no-op.
func (this *DefaultExtender) Visited(u *url.URL, harvested []*url.URL) {}

// Disallowed is a no-op.
func (this *DefaultExtender) Disallowed(u *url.URL) {}
