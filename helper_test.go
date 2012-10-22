package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"log"
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
	eMKFilter
	eMKEnqueued
	eMKVisit
	eMKVisited
	eMKDisallowed
)

// The file fetcher, that loads URLs from files in the testdata/ directory.
type fileFetcherExtender struct {
	*DefaultExtender
}

func newFileFetcher(defExt *DefaultExtender) *fileFetcherExtender {
	return &fileFetcherExtender{defExt}
}

func (this *fileFetcherExtender) Fetch(u *url.URL, userAgent string) (*http.Response, error) {
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
	fileFetcherExtender
	callCount map[extensionMethodKey]int64

	v func(*http.Response, *goquery.Document) ([]*url.URL, bool)
	f func(*url.URL, *url.URL, bool) (bool, int)
}

func newSpyExtender(v func(*http.Response, *goquery.Document) ([]*url.URL, bool),
	f func(*url.URL, *url.URL, bool) (bool, int)) *spyExtender {
	return &spyExtender{fileFetcherExtender{}, make(map[extensionMethodKey]int64, 11), v, f}
}

func newSpyExtenderConfigured(visitDelay time.Duration, returnUrls []*url.URL, doLinks bool,
	filterDelay time.Duration, filterWhitelist ...string) *spyExtender {

	v := func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(visitDelay)
		return returnUrls, doLinks
	}

	f := func(target *url.URL, origin *url.URL, isVisited bool) (bool, int) {
		time.Sleep(filterDelay)
		if len(filterWhitelist) == 1 && filterWhitelist[0] == "*" {
			// Allow all, unless already visited
			return !isVisited, 0
		} else if len(filterWhitelist) > 0 {
			if indexInStrings(filterWhitelist, target.String()) >= 0 {
				// Allow if whitelisted and not already visited
				return !isVisited, 0
			}
		}
		return false, 0
	}
	return newSpyExtender(v, f)
}

func (this *spyExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
	this.callCount[eMKVisit]++
	if this.v == nil {
		return this.fileFetcherExtender.Visit(res, doc)
	}
	return this.v(res, doc)
}

func (this *spyExtender) Filter(target *url.URL, origin *url.URL, isVisited bool) (bool, int) {
	this.callCount[eMKFilter]++
	if this.f == nil {
		return this.fileFetcherExtender.Filter(target, origin, isVisited)
	}
	return this.f(target, origin, isVisited)
}

func (this *spyExtender) Start(seeds []string) []string {
	this.callCount[eMKStart]++
	return this.fileFetcherExtender.Start(seeds)
}
func (this *spyExtender) End(reason EndReason) {
	this.callCount[eMKEnd]++
	this.fileFetcherExtender.End(reason)
}
func (this *spyExtender) Error(err *CrawlError) {
	this.callCount[eMKError]++
	this.fileFetcherExtender.Error(err)
}
func (this *spyExtender) ComputeDelay(host string, optsDelay time.Duration,
	robotsDelay time.Duration, lastFetch time.Duration) time.Duration {
	this.callCount[eMKComputeDelay]++
	return this.fileFetcherExtender.ComputeDelay(host, optsDelay, robotsDelay, lastFetch)
}
func (this *spyExtender) Fetch(u *url.URL, userAgent string) (res *http.Response, err error) {
	this.callCount[eMKFetch]++
	return this.fileFetcherExtender.Fetch(u, userAgent)
}
func (this *spyExtender) RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte) {
	this.callCount[eMKRequestRobots]++
	return this.fileFetcherExtender.RequestRobots(u, robotAgent)
}
func (this *spyExtender) Enqueued(u *url.URL, from *url.URL) {
	this.callCount[eMKEnqueued]++
	this.fileFetcherExtender.Enqueued(u, from)
}
func (this *spyExtender) Visited(u *url.URL, harvested []*url.URL) {
	this.callCount[eMKVisited]++
	this.fileFetcherExtender.Visited(u, harvested)
}
func (this *spyExtender) Disallowed(u *url.URL) {
	this.callCount[eMKDisallowed]++
	this.fileFetcherExtender.Disallowed(u)
}

func runFileFetcherWithOptions(opts *Options, urlSel []string, seeds []string) (spy *spyExtender, b *bytes.Buffer) {
	// Initialize log, spies and crawler
	b = new(bytes.Buffer)
	spy = newSpyExtenderConfigured(0, nil, true, 0, urlSel...)

	opts.Extender = spy
	opts.Logger = log.New(b, "", 0)
	c := NewCrawlerWithOptions(opts)

	c.Run(seeds...)
	return spy, b
}

func assertIsInLog(buf bytes.Buffer, s string, t *testing.T) {
	if lg := buf.String(); !strings.Contains(lg, s) {
		t.Errorf("Expected log to contain %s.", s)
		t.Logf("Log is: %s", lg)
	}
}

func assertCallCount(spy *spyExtender, key extensionMethodKey, i int64, t *testing.T) {
	if spy.callCount[key] != i {
		t.Errorf("Expected %d call count, got %d.", i, spy.callCount[key])
	}
}

func assertPanic(t *testing.T) {
	if e := recover(); e == nil {
		t.Error("Expected a panic.")
	}
}
