package gocrawl

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"log"
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
	CekParseRedirectUrl
)

// Flag indicating the head request override mode
type HeadRequestMode uint8

const (
	HrmDefault HeadRequestMode = iota
	HrmRequest
	HrmIgnore
)

// Crawl error information.
type CrawlError struct {
	Err  error
	Kind CrawlErrorKind
	URL  *url.URL
	msg  string
}

// Implementation of the error interface.
func (this CrawlError) Error() string {
	if this.Err != nil {
		return this.Err.Error()
	}
	return this.msg
}

// Create a new CrawlError based on a source error.
func newCrawlError(e error, kind CrawlErrorKind, u *url.URL) *CrawlError {
	return &CrawlError{e, kind, u, ""}
}

// Create a new CrawlError with the specified message.
func newCrawlErrorMessage(msg string, kind CrawlErrorKind, u *url.URL) *CrawlError {
	return &CrawlError{nil, kind, u, msg}
}

// Delay information: the Options delay, the Robots.txt delay, and the last delay used.
type DelayInfo struct {
	OptsDelay   time.Duration
	RobotsDelay time.Duration
	LastDelay   time.Duration
}

// Fetch information: the duration of the fetch, the returned status code, whether or
// not it was a HEAD request, and whether or not it was a robots.txt request.
type FetchInfo struct {
	Duration      time.Duration
	StatusCode    int
	HeadRequest   bool
	RobotsRequest bool
}

// Extension methods required to provide an extender instance.
type Extender interface {
	Start([]string) []string
	End(EndReason)
	Error(*CrawlError)
	Log(LogFlags, LogFlags, string)

	ComputeDelay(string, *DelayInfo, *FetchInfo) time.Duration
	Fetch(*url.URL, string, bool) (*http.Response, error)
	RequestGet(*http.Response) bool
	RequestRobots(*url.URL, string) (bool, []byte)
	FetchedRobots(*http.Response)

	Filter(*url.URL, *url.URL, bool, EnqueueOrigin) (bool, int, HeadRequestMode)
	Enqueued(*url.URL, *url.URL)
	Visit(*http.Response, *goquery.Document) ([]*url.URL, bool)
	Visited(*url.URL, []*url.URL)
	Disallowed(*url.URL)
}

// The error type returned when a redirection is requested, so that the
// worker knows that this is not an actual Fetch error, but a request to
// enqueue the redirect-to URL.
type EnqueueRedirectError struct {
	msg string
}

// Implement the error interface
func (this *EnqueueRedirectError) Error() string {
	return this.msg
}

// The HTTP client used by all fetch requests (this is thread-safe)
var httpClient = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
	// For robots.txt URLs, allow up to 10 redirects, like the default http client.
	// Rationale: the site owner explicitly tells us that this specific robots.txt
	// should be used for this domain.
	if isRobotsTxtUrl(req.URL) {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}

	// For all other URLs, do NOT follow redirections, the default Fetch() implementation
	// will ask the worker to enqueue the new (redirect-to) URL. Returning an error
	// will make httpClient.Do() return a url.Error, with the URL field containing the new URL.
	return &EnqueueRedirectError{"redirection not followed"}
}}

// Default working implementation of an extender.
type DefaultExtender struct {
	EnqueueChan chan<- *CrawlerCommand
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
func (this *DefaultExtender) Error(err *CrawlError) {}

// Log prints to the standard error by default, based on the requested log verbosity.
func (this *DefaultExtender) Log(logFlags LogFlags, msgLevel LogFlags, msg string) {
	if logFlags&msgLevel == msgLevel {
		log.Println(msg)
	}
}

// ComputeDelay returns the delay specified in the Crawler's Options, unless a
// crawl-delay is specified in the robots.txt file, which has precedence.
func (this *DefaultExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	if di.RobotsDelay > 0 {
		return di.RobotsDelay
	}
	return di.OptsDelay
}

// Fetch requests the specified URL using the given user agent string. It uses
// a custom http Client instance that doesn't follow redirections. Instead, the
// redirected-to URL is enqueued so that it goes through the same Filter() and
// Fetch() process as any other URL.
//
// Two options were considered for the default Fetch() implementation :
// 1- Not following any redirections, and enqueuing the redirect-to URL,
//    failing the current call with the 3xx status code.
// 2- Following all redirections, enqueuing only the last one (where redirection
//    stops). Returning the response of the next-to-last request.
//
// Ultimately, 1) was implemented, as it is the most generic solution that makes
// sense as default for the library. It involves no "magic" and gives full control
// as to what can happen, with the disadvantage of having the Filter() being aware
// of all possible intermediary URLs before reaching the final destination of
// a redirection (i.e. if A redirects to B that redirects to C, Filter has to
// allow A, B, and C to be Fetched, while solution 2 would only have required
// Filter to allow A and C).
//
// Solution 2) also has the disadvantage of fetching twice the final URL (once 
// while processing the original URL, so that it knows that there is no more
// redirection HTTP code, and another time when the actual destination URL is
// fetched to be visited).
func (this *DefaultExtender) Fetch(u *url.URL, userAgent string, headRequest bool) (*http.Response, error) {
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
	return httpClient.Do(req)
}

// Ask the worker to actually request the URL's body (issue a GET), unless
// the status code is not 2xx.
func (this *DefaultExtender) RequestGet(headRes *http.Response) bool {
	return headRes.StatusCode >= 200 && headRes.StatusCode < 300
}

// Ask the worker to actually request (fetch) the Robots.txt document.
func (this *DefaultExtender) RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte) {
	return true, nil
}

// FetchedRobots is a no-op.
func (this *DefaultExtender) FetchedRobots(res *http.Response) {}

// Enqueue the URL if it hasn't been visited yet.
func (this *DefaultExtender) Filter(u *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (enqueue bool, priority int, headRequest HeadRequestMode) {
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
