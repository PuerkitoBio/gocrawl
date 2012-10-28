package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAllSameHost(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spy, eMKVisit, 5, t)
	assertCallCount(spy, eMKFilter, 13, t)
}

func TestAllNotSameHost(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spy, eMKVisit, 10, t)
	assertCallCount(spy, eMKFilter, 24, t)
}

func TestSelectOnlyPage1s(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy, _ := runFileFetcherWithOptions(opts,
		[]string{"http://hosta/page1.html", "http://hostb/page1.html", "http://hostc/page1.html", "http://hostd/page1.html"},
		[]string{"http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html"})

	assertCallCount(spy, eMKVisit, 3, t)
	assertCallCount(spy, eMKFilter, 11, t)
}

func TestRunTwiceSameInstance(t *testing.T) {
	spy := newSpyExtenderConfigured(0, nil, true, 0, "*")

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogNone
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html", "http://hosta/page4.html")

	assertCallCount(spy, eMKVisit, 5, t)
	assertCallCount(spy, eMKFilter, 13, t)

	spy = newSpyExtenderConfigured(0, nil, true, 0, "http://hosta/page1.html", "http://hostb/page1.html", "http://hostc/page1.html", "http://hostd/page1.html")
	opts.SameHostOnly = false
	opts.Extender = spy
	c.Run("http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html")

	assertCallCount(spy, eMKVisit, 3, t)
	assertCallCount(spy, eMKFilter, 11, t)
}

func TestIdleTimeOut(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.WorkerIdleTTL = 200 * time.Millisecond
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogInfo
	_, b := runFileFetcherWithOptions(opts,
		[]string{"*"},
		[]string{"http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html"})

	assertIsInLog(*b, "worker for host hostd cleared on idle policy\n", t)
	assertIsInLog(*b, "worker for host hostunknown cleared on idle policy\n", t)
}

func TestReadBodyInVisitor(t *testing.T) {
	var err error
	var b []byte

	spy := newSpyExtenderFunc(eMKVisit, func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		b, err = ioutil.ReadAll(res.Body)
		return nil, false
	})

	c := NewCrawler(spy)
	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogAll
	c.Run("http://hostc/page3.html")

	if err != nil {
		t.Error(err)
	} else if len(b) == 0 {
		t.Error("Empty body")
	}
}

func TestEnqueuedCount(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robota/page1.html"})

	// page1 and robots.txt (did not visit page1, so page2 never found)
	assertCallCount(spy, eMKEnqueued, 2, t)
	// No visit per robots policy
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestVisitedCount(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html"})

	assertCallCount(spy, eMKVisited, 3, t)
}

func TestStartExtender(t *testing.T) {
	spy := newSpyExtenderFunc(eMKStart, func(seeds []string) []string {
		return append(seeds, "http://hostb/page1.html")
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostc/page1.html")

	assertCallCount(spy, eMKStart, 1, t)
	assertCallCount(spy, eMKVisit, 4, t)
	// Page1-2 for both, robots a-b, page unknown
	assertCallCount(spy, eMKEnqueued, 7, t)
}

func TestEndReasonMaxVisits(t *testing.T) {
	var e EndReason

	spy := newSpyExtenderFunc(eMKEnd, func(end EndReason) {
		e = end
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.MaxVisits = 1
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	assertCallCount(spy, eMKEnd, 1, t)
	if e != ErMaxVisits {
		t.Fatalf("Expected end reason MaxVisits, got %v\n", e)
	}
}

func TestEndReasonDone(t *testing.T) {
	var e EndReason

	spy := newSpyExtenderFunc(eMKEnd, func(end EndReason) {
		e = end
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page5.html")

	assertCallCount(spy, eMKEnd, 1, t)
	if e != ErDone {
		t.Fatalf("Expected end reason Done, got %v\n", e)
	}
}

func TestErrorFetch(t *testing.T) {
	var e *CrawlError

	spy := newSpyExtenderFunc(eMKError, func(err *CrawlError) {
		e = err
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostb/page1.html")

	assertCallCount(spy, eMKError, 1, t)
	if e.Kind != CekFetch {
		t.Fatalf("Expected error kind Fetch, got %v\n", e.Kind)
	}
}

func TestComputeDelay(t *testing.T) {
	b := new(bytes.Buffer)

	spy := newSpyExtenderFunc(eMKComputeDelay, func(host string, optsDelay time.Duration, robotsDelay time.Duration, lastFetch time.Duration) time.Duration {
		return 17 * time.Millisecond
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.Logger = log.New(b, "", 0)
	opts.LogFlags = LogError | LogInfo
	opts.MaxVisits = 2
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	assertCallCount(spy, eMKComputeDelay, 3, t)
	assertIsInLog(*b, "using crawl-delay: 17ms\n", t)
}

func TestFilter(t *testing.T) {
	b := new(bytes.Buffer)

	spy := newSpyExtenderFunc(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int) {
		return strings.HasSuffix(u.Path, "page1.html"), 0
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.Logger = log.New(b, "", 0)
	opts.LogFlags = LogError | LogIgnored
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostc/page1.html")

	assertCallCount(spy, eMKFilter, 3, t)
	assertCallCount(spy, eMKEnqueued, 2, t) // robots.txt triggers Enqueued too
	assertIsInLog(*b, "ignore on filter policy: http://hostc/page2.html\n", t)
}
