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
	"sync/atomic"
	"testing"
	"time"
)

const (
	DefaultTestCrawlDelay = 100 * time.Millisecond
)

type spyExtender struct {
	DefaultExtender
	visitCallCount  int64
	filterCallCount int64
	basePath        string

	v func(*http.Response, *goquery.Document) ([]*url.URL, bool)
	f func(*url.URL, *url.URL, bool) (bool, int)
}

func (this *spyExtender) resetCallCounts() {
	this.visitCallCount = 0
	this.filterCallCount = 0
}

func (this *spyExtender) configureVisit(delay time.Duration, urls []*url.URL, ret bool) {
	this.v = func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		this.visitCallCount = atomic.AddInt64(&this.visitCallCount, 1)
		time.Sleep(delay)
		return urls, ret
	}
}

func (this *spyExtender) configureFilter(delay time.Duration, whitelist ...string) {
	this.f = func(target *url.URL, origin *url.URL, isVisited bool) (bool, int) {
		this.filterCallCount = atomic.AddInt64(&this.filterCallCount, 1)
		time.Sleep(delay)
		if len(whitelist) == 1 && whitelist[0] == "*" {
			// Allow all, unless already visited
			return !isVisited, 0
		} else if len(whitelist) > 0 {
			if indexInStrings(whitelist, target.String()) >= 0 {
				// Allow if whitelisted and not already visited
				return !isVisited, 0
			}
		}
		return false, 0
	}
}

func (this *spyExtender) configureFetch(basePath string) {
	this.basePath = basePath
}

func (this *spyExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
	if this.v == nil {
		return this.DefaultExtender.Visit(res, doc)
	}
	return this.v(res, doc)
}

func (this *spyExtender) Filter(target *url.URL, origin *url.URL, isVisited bool) (bool, int) {
	if this.f == nil {
		return this.DefaultExtender.Filter(target, origin, isVisited)
	}
	return this.f(target, origin, isVisited)
}

func (this *spyExtender) Fetch(u *url.URL, userAgent string) (*http.Response, error) {
	var res *http.Response = new(http.Response)
	var req *http.Request
	var e error

	if req, e = http.NewRequest("GET", u.String(), nil); e != nil {
		panic(e)
	}

	// Prepare the pseudo-request
	req.Header.Add("User-Agent", userAgent)

	// Open the file specified as path in u, relative to testdata/[host]/
	f, e := os.Open(path.Join(this.basePath, u.Host, u.Path))
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

func runFileFetcherWithOptions(opts *Options, urlSel []string, seeds []string) (spy *spyExtender, b *bytes.Buffer) {
	// Initialize log, spies and crawler
	b = new(bytes.Buffer)
	spy = new(spyExtender)
	spy.configureFilter(0, urlSel...)
	spy.configureVisit(0, nil, true)
	spy.configureFetch("./testdata/")

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

func assertCallCount(c int64, i int64, t *testing.T) {
	if c != i {
		t.Errorf("Expected %d call count, got %d.", i, c)
	}
}
