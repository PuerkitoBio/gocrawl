package gocrawl

/*
import (
	"testing"
	"time"
)

func TestNoCrawlDelay(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = 0
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

  // TODO : Elapsed time < 10ms?
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

func TestLongCrawlDelayHighCpu(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = 10 * time.Second
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html"})

	// TODO : Assert high CPU usage from within the test? Profiling is kind of broken on OSX...
	assertCallCount(spy, eMKVisit, 3, t)
	assertCallCount(spy, eMKFilter, 10, t)
}
*/
