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
