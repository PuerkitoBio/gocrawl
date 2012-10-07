package gocrawl

import (
	"bytes"
	"log"
	"testing"
)

func xTestBasicRealHttpRequests(t *testing.T) {
	spy := newVisitorSpy(0, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.MaxVisits = 5
	c.Options.SameHostOnly = false
	c.Options.LogFlags = LogError | LogTrace

	c.Run("http://provok.in")
}

func TestInvalidSeed(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(0, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = DefaultCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	c.Run("#toto")
	assertIsInLog(b, "Error parsing seed URL", t)
	assertCallCount(spy, 0, t)
}

func TestHostCount(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(0, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	// Use ftp scheme so that it doesn't actually attempt a fetch
	c.Run("ftp://roota/a", "ftp://roota/b", "ftp://rootb/c")
	assertIsInLog(b, "Initial host count is 2", t)
	assertIsInLog(b, "Parsed seeds length: 3", t)
	assertCallCount(spy, 0, t)
}

func TestCustomSelectorNoUrl(t *testing.T) {
	var b bytes.Buffer

	vspy := newVisitorSpy(0, nil, true)
	uspy := newUrlSelectorSpy(0)
	c := NewCrawler(vspy.f, uspy.f)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run("http://test1", "http://test2")
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test1", t)
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test2", t)
	assertCallCount(uspy, 2, t)
	assertCallCount(vspy, 0, t)
}

func TestNoSeed(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(0, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run()
	assertCallCount(spy, 0, t)
}

func TestNoVisitorFunc(t *testing.T) {
	var b bytes.Buffer

	opts := NewOptions(nil, nil)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	opts.Fetcher = newFileFetcher("./testdata/")
	opts.Logger = log.New(&b, "", 0)

	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")
	assertIsInLog(b, "No visitor function, url not visited http://hosta/page1.html", t)
}
