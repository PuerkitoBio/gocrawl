package gocrawl

import (
	"bytes"
	"log"
	"testing"
)

func xTestBasicRealHttpRequests(t *testing.T) {
	spy := new(spyExtender)
	spy.configureVisit(0, nil, true)
	c := NewCrawler(spy.Visit, nil)

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.MaxVisits = 5
	c.Options.SameHostOnly = false
	c.Options.LogFlags = LogError | LogTrace

	c.Run("http://provok.in")
}

func TestInvalidSeed(t *testing.T) {
	var b bytes.Buffer

	spy := new(spyExtender)
	spy.configureVisit(0, nil, true)
	c := NewCrawler(spy.Visit, nil)

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	c.Run("#toto")
	assertIsInLog(b, "ERROR parsing seed #toto\n", t)
	assertCallCount(spy.visitCallCount, 0, t)
}

func TestHostCount(t *testing.T) {
	var b bytes.Buffer

	spy := new(spyExtender)
	spy.configureVisit(0, nil, true)
	c := NewCrawler(spy.Visit, nil)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	// Use ftp scheme so that it doesn't actually attempt a fetch
	c.Run("ftp://roota/a", "ftp://roota/b", "ftp://rootb/c")
	assertIsInLog(b, "init() - host count: 2\n", t)
	assertIsInLog(b, "init() - seeds length: 3\n", t)
	assertCallCount(spy.visitCallCount, 0, t)
}

func TestCustomSelectorNoUrl(t *testing.T) {
	var b bytes.Buffer

	spy := new(spyExtender)
	spy.configureVisit(0, nil, true)
	spy.configureFilter(0)
	c := NewCrawler(spy.Visit, spy.Filter)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogIgnored
	c.Options.Logger = log.New(&b, "", 0)
	c.Run("http://test1", "http://test2")
	assertIsInLog(b, "ignore on custom selector policy: http://test1\n", t)
	assertIsInLog(b, "ignore on custom selector policy: http://test2\n", t)
	assertCallCount(spy.filterCallCount, 2, t)
	assertCallCount(spy.visitCallCount, 0, t)
}

func TestNoSeed(t *testing.T) {
	var b bytes.Buffer

	spy := new(spyExtender)
	spy.configureVisit(0, nil, true)
	c := NewCrawler(spy.Visit, nil)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run()
	assertCallCount(spy.visitCallCount, 0, t)
}

func TestNoVisitorFunc(t *testing.T) {
	var b bytes.Buffer

	spy := new(spyExtender)
	spy.configureFetch("./testdata/")

	opts := NewOptions(nil, nil)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	opts.Logger = log.New(&b, "", 0)
	opts.Extender = spy

	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")
	assertIsInLog(b, "missing visitor function: http://hosta/page1.html\n", t)
}

func TestNoCrawlDelay(t *testing.T) {
	var b bytes.Buffer

	opts := NewOptions(nil, nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = 0
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spy.visitCallCount, 5, t)
	assertCallCount(spy.filterCallCount, 13, t)
}
