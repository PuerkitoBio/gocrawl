package gocrawl

import (
	"testing"
)

func xTestBasicRealHttpRequests(t *testing.T) {
	c := NewCrawler(new(DefaultExtender))

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.MaxVisits = 5
	c.Options.SameHostOnly = false
	c.Options.LogFlags = LogError | LogTrace | LogInfo

	c.Run("http://provok.in")
}

func TestNonHtmlRequest(t *testing.T) {
	c := NewCrawler(new(DefaultExtender))

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError

	if er := c.Run("https://lh4.googleusercontent.com/-v0soe-ievYE/AAAAAAAAAAI/AAAAAAAAs7Y/_UbxpxC-VG0/photo.jpg"); er != ErDone {
		t.Fatalf("Expected end reason %s, got %s", ErDone, er)
	}
}

func TestInvalidSeed(t *testing.T) {
	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.LogFlags = LogError

	c.Run("#toto")
	assertIsInLog(spy.b, "ERROR parsing seed #toto\n", t)
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestHostCount(t *testing.T) {
	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace

	// Use ftp scheme so that it doesn't actually attempt a fetch
	c.Run("ftp://roota/a", "ftp://roota/b", "ftp://rootb/c")
	assertIsInLog(spy.b, "init() - host count: 2\n", t)
	assertIsInLog(spy.b, "init() - seeds length: 3\n", t)
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestCustomSelectorNoUrl(t *testing.T) {
	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogIgnored
	c.Run("http://test1", "http://test2")
	assertIsInLog(spy.b, "ignore on filter policy: http://test1\n", t)
	assertIsInLog(spy.b, "ignore on filter policy: http://test2\n", t)
	assertCallCount(spy, eMKFilter, 2, t)
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestNoSeed(t *testing.T) {
	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Run()
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestNoVisitorFunc(t *testing.T) {
	spy := newSpyExtender(nil, nil)
	opts := NewOptions(spy)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	opts.SameHostOnly = true

	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	// Without a specific visit and filter functions, will visit all links found
	assertCallCount(spy, eMKVisit, 3, t)
	assertCallCount(spy, eMKFilter, 10, t)
}

func TestNoCrawlDelay(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = 0
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spy, eMKVisit, 5, t)
	assertCallCount(spy, eMKFilter, 13, t)
}

func TestNoExtender(t *testing.T) {
	defer assertPanic(t)

	c := NewCrawler(nil)
	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Run()
}
