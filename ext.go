package gocrawl

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"time"
)

// Delay information: the Options delay, the Robots.txt delay, and the last delay used.
type DelayInfo struct {
	OptsDelay   time.Duration
	RobotsDelay time.Duration
	LastDelay   time.Duration
}

// Fetch information: the duration of the fetch, the returned status code, whether or
// not it was a HEAD request, and whether or not it was a robots.txt request.
type FetchInfo struct {
	Ctx           *URLContext
	Duration      time.Duration
	StatusCode    int
	IsHeadRequest bool
}

// Extension methods required to provide an extender instance.
type Extender interface {
	// Start, End, Error and Log are not related to a specific URL, so they don't
	// receive a URLContext struct.
	Start(interface{}) interface{}
	End(error)
	Error(*CrawlError)
	Log(LogFlags, LogFlags, string)

	// ComputeDelay is related to a Host only, not to a URLContext, although the FetchInfo
	// is related to a URLContext (holds a ctx field).
	ComputeDelay(string, *DelayInfo, *FetchInfo) time.Duration

	// All other extender methods are executed in the context of an URL, and thus
	// receive an URLContext struct as first argument.
	Fetch(*URLContext, string, bool) (*http.Response, error)
	RequestGet(*URLContext, *http.Response) bool
	RequestRobots(*URLContext, string) ([]byte, bool)
	FetchedRobots(*URLContext, *http.Response)
	Filter(*URLContext, bool) bool
	Enqueued(*URLContext)
	Visit(*URLContext, *http.Response, *goquery.Document) (interface{}, bool)
	Visited(*URLContext, interface{})
	Disallowed(*URLContext)
}

// The default HTTP client used by DefaultExtender's fetch requests (this is thread-safe).
// The client's fields can be customized (i.e. for a different redirection strategy, a
// different Transport object, ...). It should be done prior to starting the crawler.
var HttpClient = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
	// For robots.txt URLs, allow up to 10 redirects, like the default http client.
	// Rationale: the site owner explicitly tells us that this specific robots.txt
	// should be used for this domain.
	if isRobotsURL(req.URL) {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}

	// For all other URLs, do NOT follow redirections, the default Fetch() implementation
	// will ask the worker to enqueue the new (redirect-to) URL. Returning an error
	// will make httpClient.Do() return a url.Error, with the URL field containing the new URL.
	return ErrEnqueueRedirect
}}

// Default working implementation of an extender.
type DefaultExtender struct {
	EnqueueChan chan<- interface{}
}

// Return the same seeds as those received (those that were passed
// to Run() initially).
func (this *DefaultExtender) Start(seeds interface{}) interface{} {
	return seeds
}

// End is a no-op.
func (this *DefaultExtender) End(err error) {}

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
func (this *DefaultExtender) Fetch(ctx *URLContext, userAgent string, headRequest bool) (*http.Response, error) {
	var reqType string

	// Prepare the request with the right user agent
	if headRequest {
		reqType = "HEAD"
	} else {
		reqType = "GET"
	}
	req, e := http.NewRequest(reqType, ctx.url.String(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", userAgent)
	return HttpClient.Do(req)
}

// Ask the worker to actually request the URL's body (issue a GET), unless
// the status code is not 2xx.
func (this *DefaultExtender) RequestGet(ctx *URLContext, headRes *http.Response) bool {
	return headRes.StatusCode >= 200 && headRes.StatusCode < 300
}

// Ask the worker to actually request (fetch) the Robots.txt document.
func (this *DefaultExtender) RequestRobots(ctx *URLContext, robotAgent string) (data []byte, doRequest bool) {
	return nil, true
}

// FetchedRobots is a no-op.
func (this *DefaultExtender) FetchedRobots(ctx *URLContext, res *http.Response) {}

// Enqueue the URL if it hasn't been visited yet.
func (this *DefaultExtender) Filter(ctx *URLContext, isVisited bool) bool {
	return !isVisited
}

// Enqueued is a no-op.
func (this *DefaultExtender) Enqueued(ctx *URLContext) {}

// Ask the worker to harvest the links in this page.
func (this *DefaultExtender) Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (harvested interface{}, findLinks bool) {
	return nil, true
}

// Visited is a no-op.
func (this *DefaultExtender) Visited(ctx *URLContext, harvested interface{}) {}

// Disallowed is a no-op.
func (this *DefaultExtender) Disallowed(ctx *URLContext) {}
