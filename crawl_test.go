package gocrawl

import (
	"testing"
	"time"
)

func TestAllSameHost(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spyv, 5, t)
	assertCallCount(spyu, 13, t)
}

func TestAllNotSameHost(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spyv, 10, t)
	assertCallCount(spyu, 24, t)
}

func TestSelectOnlyPage1s(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts,
		[]string{"http://hosta/page1.html", "http://hostb/page1.html", "http://hostc/page1.html", "http://hostd/page1.html"},
		[]string{"http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html"})

	assertCallCount(spyv, 3, t)
	assertCallCount(spyu, 11, t)
}

func TestRunTwiceSameInstance(t *testing.T) {
	spyv := newVisitorSpy(0, nil, true)
	spyu := newUrlSelectorSpy(0, "*")

	opts := NewOptions(nil, nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.URLVisitor = spyv.f
	opts.URLSelector = spyu.f
	opts.LogFlags = LogNone
	opts.Fetcher = newFileFetcher("./testdata/")
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html", "http://hosta/page4.html")

	assertCallCount(spyv, 5, t)
	assertCallCount(spyu, 13, t)

	spyv = newVisitorSpy(0, nil, true)
	spyu = newUrlSelectorSpy(0, "http://hosta/page1.html", "http://hostb/page1.html", "http://hostc/page1.html", "http://hostd/page1.html")
	opts.URLVisitor = spyv.f
	opts.URLSelector = spyu.f
	opts.SameHostOnly = false
	c.Run("http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html")

	assertCallCount(spyv, 3, t)
	assertCallCount(spyu, 11, t)
}

func TestIdleTimeOut(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.WorkerIdleTTL = 200 * time.Millisecond
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	_, _, b := runFileFetcherWithOptions(opts,
		[]string{"*"},
		[]string{"http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html"})

	assertIsInLog(*b, "Cleared idle worker for host hostd", t)
	assertIsInLog(*b, "Cleared idle worker for host hostunknown", t)
}
