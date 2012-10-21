package gocrawl

import (
	"bytes"
	"log"
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

func TestInvalidSeed(t *testing.T) {
	var b bytes.Buffer

	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.LogFlags = LogError
	c.Options.Logger = log.New(&b, "", 0)

	c.Run("#toto")
	assertIsInLog(b, "ERROR parsing seed #toto\n", t)
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestHostCount(t *testing.T) {
	var b bytes.Buffer

	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	// Use ftp scheme so that it doesn't actually attempt a fetch
	c.Run("ftp://roota/a", "ftp://roota/b", "ftp://rootb/c")
	assertIsInLog(b, "init() - host count: 2\n", t)
	assertIsInLog(b, "init() - seeds length: 3\n", t)
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestCustomSelectorNoUrl(t *testing.T) {
	var b bytes.Buffer

	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogIgnored
	c.Options.Logger = log.New(&b, "", 0)
	c.Run("http://test1", "http://test2")
	assertIsInLog(b, "ignore on filter policy: http://test1\n", t)
	assertIsInLog(b, "ignore on filter policy: http://test2\n", t)
	assertCallCount(spy, eMKFilter, 2, t)
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestNoSeed(t *testing.T) {
	var b bytes.Buffer

	spy := newSpyExtenderConfigured(0, nil, true, 0)
	c := NewCrawler(spy)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run()
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestNoVisitorFunc(t *testing.T) {
	var b bytes.Buffer

	spy := newSpyExtender(nil, nil)
	opts := NewOptions(spy)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	opts.Logger = log.New(&b, "", 0)
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
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spy, eMKVisit, 5, t)
	assertCallCount(spy, eMKFilter, 13, t)
}

func TestNoExtender(t *testing.T) {
	var b bytes.Buffer

	defer assertPanic(t)

	c := NewCrawler(nil)
	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run()
}
