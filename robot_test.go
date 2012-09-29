package gocrawl

import (
	"testing"
	"time"
)

func TestRobotDenyAll(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robota/page1.html"})

	assertCallCount(spyv, 0, t)
	assertCallCount(spyu, 1, t)
}

func TestRobotPartialDenyGooglebot(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotb/page1.html"})

	assertCallCount(spyv, 2, t)
	assertCallCount(spyu, 4, t)
}

func TestRobotDenyOtherBot(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogError | LogTrace
	opts.RobotUserAgent = "NotGoogleBot"
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotb/page1.html"})

	assertCallCount(spyv, 4, t)
	assertCallCount(spyu, 5, t)
}

/*
// TODO : This test fails because of a bug in robotstxt.go library, will try to fix and send a pull request.
func TestRobotExplicitAllowPattern(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotc/page1.html"})

	assertCallCount(spyv, 4, t)
	assertCallCount(spyu, 5, t)
}
*/
