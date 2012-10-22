package gocrawl

import (
	"testing"
)

func TestRobotDenyAll(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robota/page1.html"})

	assertCallCount(spy, eMKVisit, 0, t)
	assertCallCount(spy, eMKFilter, 1, t)
}

func TestRobotPartialDenyGooglebot(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotb/page1.html"})

	assertCallCount(spy, eMKVisit, 2, t)
	assertCallCount(spy, eMKFilter, 4, t)
}

func TestRobotDenyOtherBot(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	opts.RobotUserAgent = "NotGoogleBot"
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotb/page1.html"})

	assertCallCount(spy, eMKVisit, 4, t)
	assertCallCount(spy, eMKFilter, 5, t)
}

func TestRobotExplicitAllowPattern(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotc/page1.html"})

	assertCallCount(spy, eMKVisit, 4, t)
	assertCallCount(spy, eMKFilter, 5, t)
}

func TestRobotCrawlDelay(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	spy, b := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotc/page1.html"})

	assertCallCount(spy, eMKVisit, 4, t)
	assertCallCount(spy, eMKFilter, 5, t)
	assertIsInLog(*b, "using crawl-delay: 200ms\n", t)
}
