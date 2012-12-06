package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

const (
	DefaultTestCrawlDelay = 100 * time.Millisecond
	FileFetcherBasePath   = "./testdata/"
)

type extensionMethodKey uint8

const (
	eMKStart extensionMethodKey = iota
	eMKEnd
	eMKError
	eMKComputeDelay
	eMKFetch
	eMKRequestRobots
	eMKRequestGet
	eMKFetchedRobots
	eMKFilter
	eMKEnqueued
	eMKVisit
	eMKVisited
	eMKDisallowed
	eMKLast
)

// The file fetcher, that loads URLs from files in the testdata/ directory.
type fileFetcherExtender struct {
	*DefaultExtender
}

func newFileFetcher(defExt *DefaultExtender) *fileFetcherExtender {
	return &fileFetcherExtender{defExt}
}

func (this *fileFetcherExtender) Fetch(u *url.URL, userAgent string, headRequest bool) (*http.Response, error) {
	var res *http.Response = new(http.Response)
	var req *http.Request
	var e error

	if req, e = http.NewRequest("GET", u.String(), nil); e != nil {
		panic(e)
	}

	// Prepare the pseudo-request
	req.Header.Add("User-Agent", userAgent)

	// Open the file specified as path in u, relative to testdata/[host]/
	f, e := os.Open(path.Join(FileFetcherBasePath, u.Host, u.Path))
	if e != nil {
		// Treat errors as 404s - file not found
		res.Status = "404 Not Found"
		res.StatusCode = 404
	} else {
		res.Status = "200 OK"
		res.StatusCode = 200
		res.Body = f
	}
	res.Request = req

	return res, e
}

// The spy extender extends the file fetcher and allows counting the number of
// calls for each extender method.
type spyExtender struct {
	*fileFetcherExtender
	callCount map[extensionMethodKey]int64
	methods   map[extensionMethodKey]interface{}
	b         bytes.Buffer
}

type callCounter interface {
	getCallCount(extensionMethodKey) int64
	incCallCount(extensionMethodKey, int64)
}

func (this *spyExtender) getCallCount(key extensionMethodKey) int64 {
	return this.callCount[key]
}

func (this *spyExtender) incCallCount(key extensionMethodKey, delta int64) {
	this.callCount[key] += delta
}

func newSpyExtender(v func(*http.Response, *goquery.Document) ([]*url.URL, bool),
	f func(*url.URL, *url.URL, bool, EnqueueOrigin) (bool, int, HeadRequestMode)) *spyExtender {
	spy := &spyExtender{fileFetcherExtender: newFileFetcher(new(DefaultExtender)),
		callCount: make(map[extensionMethodKey]int64, eMKLast),
		methods:   make(map[extensionMethodKey]interface{}, 2)}
	if v != nil {
		spy.setExtensionMethod(eMKVisit, v)
	}
	if f != nil {
		spy.setExtensionMethod(eMKFilter, f)
	}
	return spy
}

func newSpyExtenderFunc(key extensionMethodKey, f interface{}) *spyExtender {
	spy := newSpyExtender(nil, nil)
	if f != nil {
		spy.setExtensionMethod(key, f)
	}
	return spy
}

func (this *spyExtender) setExtensionMethod(key extensionMethodKey, f interface{}) {
	this.methods[key] = f
}

func newSpyExtenderConfigured(visitDelay time.Duration, returnUrls []*url.URL, doLinks bool,
	filterDelay time.Duration, filterWhitelist ...string) *spyExtender {

	v := func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(visitDelay)
		return returnUrls, doLinks
	}

	f := func(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode) {
		time.Sleep(filterDelay)
		if len(filterWhitelist) == 1 && filterWhitelist[0] == "*" {
			// Allow all, unless already visited
			return !isVisited, 0, HrmDefault
		} else if len(filterWhitelist) > 0 {
			if indexInStrings(filterWhitelist, target.String()) >= 0 {
				// Allow if whitelisted and not already visited
				return !isVisited, 0, HrmDefault
			}
		}
		return false, 0, HrmDefault
	}
	return newSpyExtender(v, f)
}

func (this *spyExtender) Log(logFlags LogFlags, msgLevel LogFlags, msg string) {
	if logFlags&msgLevel == msgLevel {
		this.b.WriteString(msg + "\n")
	}
}

func (this *spyExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
	this.incCallCount(eMKVisit, 1)
	if f, ok := this.methods[eMKVisit].(func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool)); ok {
		return f(res, doc)
	}
	return this.fileFetcherExtender.Visit(res, doc)
}

func (this *spyExtender) Filter(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode) {
	this.incCallCount(eMKFilter, 1)
	if f, ok := this.methods[eMKFilter].(func(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode)); ok {
		return f(target, from, isVisited, origin)
	}
	return this.fileFetcherExtender.Filter(target, from, isVisited, origin)
}

func (this *spyExtender) Start(seeds []string) []string {
	this.incCallCount(eMKStart, 1)
	if f, ok := this.methods[eMKStart].(func(seeds []string) []string); ok {
		return f(seeds)
	}
	return this.fileFetcherExtender.Start(seeds)
}
func (this *spyExtender) End(reason EndReason) {
	this.incCallCount(eMKEnd, 1)
	if f, ok := this.methods[eMKEnd].(func(reason EndReason)); ok {
		f(reason)
		return
	}
	this.fileFetcherExtender.End(reason)
}
func (this *spyExtender) Error(err *CrawlError) {
	this.incCallCount(eMKError, 1)
	if f, ok := this.methods[eMKError].(func(err *CrawlError)); ok {
		f(err)
		return
	}
	this.fileFetcherExtender.Error(err)
}
func (this *spyExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	this.incCallCount(eMKComputeDelay, 1)
	if f, ok := this.methods[eMKComputeDelay].(func(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration); ok {
		return f(host, di, lastFetch)
	}
	return this.fileFetcherExtender.ComputeDelay(host, di, lastFetch)
}
func (this *spyExtender) Fetch(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
	this.incCallCount(eMKFetch, 1)
	if f, ok := this.methods[eMKFetch].(func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error)); ok {
		return f(u, userAgent, headRequest)
	}
	return this.fileFetcherExtender.Fetch(u, userAgent, headRequest)
}
func (this *spyExtender) RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte) {
	this.incCallCount(eMKRequestRobots, 1)
	if f, ok := this.methods[eMKRequestRobots].(func(u *url.URL, robotAgent string) (request bool, data []byte)); ok {
		return f(u, robotAgent)
	}
	return this.fileFetcherExtender.RequestRobots(u, robotAgent)
}
func (this *spyExtender) RequestGet(headRes *http.Response) bool {
	this.incCallCount(eMKRequestGet, 1)
	if f, ok := this.methods[eMKRequestGet].(func(headRes *http.Response) bool); ok {
		return f(headRes)
	}
	return this.fileFetcherExtender.RequestGet(headRes)
}
func (this *spyExtender) FetchedRobots(res *http.Response) {
	this.incCallCount(eMKFetchedRobots, 1)
	if f, ok := this.methods[eMKFetchedRobots].(func(res *http.Response)); ok {
		f(res)
		return
	}
	this.fileFetcherExtender.FetchedRobots(res)
}
func (this *spyExtender) Enqueued(u *url.URL, from *url.URL) {
	this.incCallCount(eMKEnqueued, 1)
	if f, ok := this.methods[eMKEnqueued].(func(u *url.URL, from *url.URL)); ok {
		f(u, from)
		return
	}
	this.fileFetcherExtender.Enqueued(u, from)
}
func (this *spyExtender) Visited(u *url.URL, harvested []*url.URL) {
	this.incCallCount(eMKVisited, 1)
	if f, ok := this.methods[eMKVisited].(func(u *url.URL, harvested []*url.URL)); ok {
		f(u, harvested)
		return
	}
	this.fileFetcherExtender.Visited(u, harvested)
}
func (this *spyExtender) Disallowed(u *url.URL) {
	this.incCallCount(eMKDisallowed, 1)
	if f, ok := this.methods[eMKDisallowed].(func(u *url.URL)); ok {
		f(u)
		return
	}
	this.fileFetcherExtender.Disallowed(u)
}

func runFileFetcherWithOptions(opts *Options, urlSel []string, seeds []string) (spy *spyExtender) {
	spy = newSpyExtenderConfigured(0, nil, true, 0, urlSel...)

	opts.Extender = spy
	c := NewCrawlerWithOptions(opts)

	c.Run(seeds...)
	return spy
}

func assertIsInLog(buf bytes.Buffer, s string, t *testing.T) {
	if lg := buf.String(); !strings.Contains(lg, s) {
		t.Errorf("Expected log to contain %s.", s)
		t.Logf("Log is: %s", lg)
	}
}

func assertCallCount(spy callCounter, key extensionMethodKey, i int64, t *testing.T) {
	cnt := spy.getCallCount(key)
	if cnt != i {
		t.Errorf("Expected %d call count, got %d.", i, cnt)
	}
}

func assertPanic(t *testing.T) {
	if e := recover(); e == nil {
		t.Error("Expected a panic.")
	}
}
