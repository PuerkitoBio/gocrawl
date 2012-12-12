package gocrawl

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)

func TestRobotDenyAll(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robota/page1.html"})

	assertCallCount(spy, eMKVisit, 0, t)
	assertCallCount(spy, eMKFilter, 1, t)
}

func TestRobotPartialDenyGooglebot(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotb/page1.html"})

	assertCallCount(spy, eMKVisit, 2, t)
	assertCallCount(spy, eMKFilter, 4, t)
}

func TestRobotDenyOtherBot(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	opts.RobotUserAgent = "NotGoogleBot"
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotb/page1.html"})

	assertCallCount(spy, eMKVisit, 4, t)
	assertCallCount(spy, eMKFilter, 5, t)
}

func TestRobotExplicitAllowPattern(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogTrace
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotc/page1.html"})

	assertCallCount(spy, eMKVisit, 4, t)
	assertCallCount(spy, eMKFilter, 5, t)
}

func TestRobotCrawlDelay(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robotc/page1.html"})

	assertCallCount(spy, eMKVisit, 4, t)
	assertCallCount(spy, eMKFilter, 5, t)
	assertIsInLog(spy.b, "using crawl-delay: 200ms\n", t)
}

func TestCachedRobot(t *testing.T) {
	spy := newSpyExtenderFunc(eMKRequestRobots, func(u *url.URL, robotAgent string) (request bool, data []byte) {
		return false, []byte("User-agent: *\nDisallow:/page2.html")
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	c := NewCrawlerWithOptions(opts)
	c.Run("http://robota/page1.html")

	assertCallCount(spy, eMKVisit, 1, t)
	assertCallCount(spy, eMKEnqueued, 3, t)
	assertCallCount(spy, eMKRequestRobots, 1, t)
	assertCallCount(spy, eMKDisallowed, 1, t)
}

func TestFetchedRobot(t *testing.T) {
	var err error
	var b []byte

	spy := newSpyExtenderFunc(eMKFetchedRobots, func(res *http.Response) {
		b, err = ioutil.ReadAll(res.Body)
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	c := NewCrawlerWithOptions(opts)
	c.Run("http://robotc/page4.html")

	assertCallCount(spy, eMKRequestRobots, 1, t)
	assertCallCount(spy, eMKEnqueued, 2, t)
	assertCallCount(spy, eMKFetchedRobots, 1, t)

	if err != nil {
		t.Error(err)
	} else if len(b) == 0 {
		t.Error("Empty body in fetched robots")
	}
}

func TestNoRobot(t *testing.T) {
	spy := newSpy(new(DefaultExtender), true)
	spy.setExtensionMethod(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		return !isVisited && from == nil, 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	c.Run("http://expressjs.com/") // TODO : Check if it really has no robots.txt!

	assertCallCount(spy, eMKFetch, 2, t) // robots + root
	assertCallCount(spy, eMKVisit, 1, t) // root
}
