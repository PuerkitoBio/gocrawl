package gocrawl

import (
	"bytes"
	"log"
	"testing"
	"time"
)

func xTestBasic(t *testing.T) {
	spy := newVisitorSpy(1*time.Millisecond, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.MaxVisits = 5
	c.Options.SameHostOnly = false
	c.Options.LogFlags = LogError | LogTrace

	c.Run("http://provok.in")
}

func TestInvalidSeed(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(1*time.Millisecond, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	c.Run("#toto")
	assertIsInLog(b, "Error parsing seed URL", t)
	assertCallCount(spy, 0, t)
}

func TestHostCount(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(1*time.Millisecond, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
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

	vspy := newVisitorSpy(1*time.Millisecond, nil, true)
	uspy := newUrlSelectorSpy(0)
	c := NewCrawler(vspy.f, uspy.f)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run("http://test1", "http://test2")
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test1", t)
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test2", t)
	assertCallCount(uspy, 2, t)
	assertCallCount(vspy, 0, t)
}

func TestNoSeed(t *testing.T) {
	spy := newVisitorSpy(0, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Run()
	assertCallCount(spy, 0, t)
}

// TODO : Use a Fetcher with static data for tests
