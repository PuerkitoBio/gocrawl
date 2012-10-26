package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"time"
)

// Flag indicating why the crawler ended.
type EndReason uint8

const (
	ErDone EndReason = iota
	ErMaxVisits
	ErError
)

// Flag indicating the source of the crawl error.
type CrawlErrorKind uint8

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

type HeadRequestMode uint8

const (
	HrmDefault HeadRequestMode = iota
	HrmRequest
	HrmIgnore
)

type CrawlError struct {
	Err  error
	Kind CrawlErrorKind
	URL  *url.URL
	msg  string
}

func (this CrawlError) Error() string {
	if this.Err != nil {
		return this.Err.Error()
	}
	return this.msg
}

func newCrawlError(e error, kind CrawlErrorKind, u *url.URL) *CrawlError {
	return &CrawlError{e, kind, u, ""}
}

func newCrawlErrorMessage(msg string, kind CrawlErrorKind, u *url.URL) *CrawlError {
	return &CrawlError{nil, kind, u, msg}
}

type DelayInfo struct {
	OptsDelay   time.Duration
	RobotsDelay time.Duration
	LastDelay   time.Duration
}

type FetchInfo struct {
	Duration      time.Duration
	StatusCode    int
	HeadRequest   bool
	RobotsRequest bool
}

// Extension methods required to provide an extender instance.
type Extender interface {
	Start(seeds []string) []string
	End(reason EndReason)
	Error(err *CrawlError)

	ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration
	Fetch(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error)
	RequestGet(headRes *http.Response) bool
	RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte)
	FetchedRobots(res *http.Response)

	Filter(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int, headRequest HeadRequestMode)
	Enqueued(u *url.URL, from *url.URL)
	Visit(*http.Response, *goquery.Document) (harvested []*url.URL, findLinks bool)
	Visited(u *url.URL, harvested []*url.URL)
	Disallowed(u *url.URL)
}

type DefaultExtender struct{}

// Return the same seeds as those received (those that were passed
// to Run() initially).
func (this *DefaultExtender) Start(seeds []string) []string {
	return seeds
}

// End is a no-op.
func (this *DefaultExtender) End(reason EndReason) {}

// Error is a no-op (logging is done automatically, regardless of the implementation
// of the Error() hook).
func (this *DefaultExtender) Error(err *CrawlError) {}

// ComputeDelay returns the delay specified in the Crawler's Options, unless a
// crawl-delay is specified in the robots.txt file, which has precedence.
func (this *DefaultExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	if di.RobotsDelay > 0 {
		return di.RobotsDelay
	}
	return di.OptsDelay
}

// Fetch requests the specified URL using the given user agent string. It uses
// Go's default http Client instance.
func (this *DefaultExtender) Fetch(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
	var reqType string

	// Prepare the request with the right user agent
	if headRequest {
		reqType = "HEAD"
	} else {
		reqType = "GET"
	}
	req, e := http.NewRequest(reqType, u.String(), nil)
	if e != nil {
		return nil, e
	}
	req.Header["User-Agent"] = []string{userAgent}
	return http.DefaultClient.Do(req)
}

// Ask the worker to actually request the URL's body (issue a GET).
func (this *DefaultExtender) RequestGet(headRes *http.Response) bool {
	return true
}

// Ask the worker to actually request (fetch) the Robots.txt document.
func (this *DefaultExtender) RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte) {
	return true, nil
}

// FetchedRobots is a no-op.
func (this *DefaultExtender) FetchedRobots(res *http.Response) {}

// Enqueue the URL if it hasn't been visited yet.
func (this *DefaultExtender) Filter(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int, headRequest HeadRequestMode) {
	return !isVisited, 0, HrmDefault
}

// Enqueued is a no-op.
func (this *DefaultExtender) Enqueued(u *url.URL, from *url.URL) {}

// Ask the worker to harvest the links in this page.
func (this *DefaultExtender) Visit(res *http.Response, doc *goquery.Document) (harvested []*url.URL, findLinks bool) {
	return nil, true
}

// Visited is a no-op.
func (this *DefaultExtender) Visited(u *url.URL, harvested []*url.URL) {}

// Disallowed is a no-op.
func (this *DefaultExtender) Disallowed(u *url.URL) {}
