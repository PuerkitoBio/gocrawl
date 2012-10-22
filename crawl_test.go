package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestAllSameHost(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
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

type bodyExtender struct {
	fileFetcherExtender

	err error
	b   []byte
}

func (this *bodyExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
	this.b, this.err = ioutil.ReadAll(res.Body)
	return nil, false
}

func TestReadBodyInVisitor(t *testing.T) {
	var be = new(bodyExtender)
	c := NewCrawler(be)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogAll
	c.Run("http://hostc/page3.html")

	if be.err != nil {
		t.Error(be.err)
	} else if len(be.b) == 0 {
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

type startExtender struct {
	*spyExtender
}

func (this *startExtender) Start(seeds []string) []string {
	this.incCallCount(eMKStart, 1)
	return append(seeds, "http://hostb/page1.html")
}

func TestStartExtender(t *testing.T) {
	spy := &startExtender{newSpyExtender(nil, nil)}
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.Extender = spy
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostc/page1.html")

	assertCallCount(spy, eMKStart, 1, t)
	assertCallCount(spy, eMKVisit, 4, t)
	// Page1-2 for both, robots a-b, page unknown
	assertCallCount(spy, eMKEnqueued, 7, t)
}
