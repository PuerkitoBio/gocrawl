package gocrawl

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testNoCrawlDelay(t *testing.T, tc *testCase, buf bool) {
	const MaxTime = 15 * time.Millisecond // Give some buffer when running with -race

	ff := newFileFetcher()
	spy := newSpy(ff, buf)
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = 0
	c := NewCrawlerWithOptions(opts)

	start := time.Now()
	c.Run([]string{
		"http://hosta/page1.html",
		"http://hosta/page4.html",
	})

	elps := time.Now().Sub(start)
	assertTrue(elps <= MaxTime, "expected elapsed time to be at most %v, got %v", MaxTime, elps)
	assertCallCount(spy, tc.name, eMKVisit, 5, t)
	assertCallCount(spy, tc.name, eMKFilter, 13, t)
}

func testNoExtender(t *testing.T, tc *testCase, buf bool) {
	defer assertPanic(tc.name, t)

	c := NewCrawler(nil)
	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogError | LogTrace

	c.Run(nil)
}

func testCrawlDelay(t *testing.T, tc *testCase, buf bool) {
	var last time.Time
	var since []time.Duration
	cnt := 0

	ff := newFileFetcher()
	spy := newSpy(ff, buf)
	spy.setExtensionMethod(eMKFetch, func(ctx *URLContext, agent string, head bool) (*http.Response, error) {
		since = append(since, time.Now().Sub(last))
		last = time.Now()
		return ff.Fetch(ctx, agent, head)
	})
	spy.setExtensionMethod(eMKComputeDelay, func(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
		// Crawl delay always grows
		cnt++
		return time.Duration(int(di.OptsDelay) * cnt)
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = true
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	last = time.Now()

	c.Run("http://hosta/page1.html")

	assertCallCount(spy, tc.name, eMKFetch, 7, t)
	assertCallCount(spy, tc.name, eMKComputeDelay, 7, t)
	for i, d := range since {
		min := (DefaultTestCrawlDelay * time.Duration(i))
		assertTrue(d >= min, "expected a delay of at least %v for fetch #%d, got %v.", min, i, d)
	}
}

func testUserAgent(t *testing.T, tc *testCase, buf bool) {
	// Create crawler, with all defaults
	c := NewCrawler(new(DefaultExtender))
	c.Options.CrawlDelay = 10 * time.Millisecond

	// Create server
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Fatal(err)
	}
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		// Expect robots.txt user agent
		assertTrue(r.UserAgent() == c.Options.RobotUserAgent, "expected user-agent %s, got %s", c.Options.RobotUserAgent, r.UserAgent())
	})
	http.HandleFunc("/bidon", func(w http.ResponseWriter, r *http.Request) {
		// Expect crawl user agent
		assertTrue(r.UserAgent() == c.Options.UserAgent, "expected user-agent %s, got %s", c.Options.UserAgent, r.UserAgent())
	})

	// Start server in a separate goroutine
	go func() {
		http.Serve(l, nil)
	}()

	// Go crawl
	c.Run("http://localhost:8080/bidon")

	// Close listener
	if err = l.Close(); err != nil {
		panic(err)
	}
}

func testRunTwiceSameInstance(t *testing.T, tc *testCase, buf bool) {
	ff := newFileFetcher()
	spy := newSpy(ff, buf)
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)

	c.Run([]string{
		"http://hosta/page1.html",
		"http://hosta/page4.html",
	})

	assertCallCount(spy, tc.name, eMKVisit, 5, t)
	assertCallCount(spy, tc.name, eMKFilter, 13, t)

	spy = newSpy(ff, buf)
	spy.setExtensionMethod(eMKFilter, func(ctx *URLContext, isVisited bool) bool {
		return !isVisited && strings.ToLower(ctx.url.Path) == "/page1.html"
	})
	opts.SameHostOnly = false
	opts.Extender = spy

	c.Run([]string{
		"http://hosta/page1.html",
		"http://hosta/page4.html",
		"http://hostb/pageunlinked.html",
	})

	assertCallCount(spy, tc.name, eMKVisit, 3, t)
	assertCallCount(spy, tc.name, eMKFilter, 11, t)
}

func testEnqueueChanEmbedded(t *testing.T, tc *testCase, buf bool) {
	type MyExt struct {
		SomeFieldBefore bool
		*DefaultExtender
		SomeFieldAfter int
	}
	me := &MyExt{false, new(DefaultExtender), 0}
	c := NewCrawler(me)
	assertTrue(me.EnqueueChan == nil, "expected EnqueueChan to be nil")

	c.Run(nil)

	assertTrue(me.EnqueueChan != nil, "expected EnqueueChan to be non-nil")
	me.EnqueueChan <- "test"
	l := len(me.EnqueueChan)
	assertTrue(l == 1, "expected EnqueueChan to have 1 element, got %d", l)
}

func testEnqueueChanShadowed(t *testing.T, tc *testCase, buf bool) {
	type ShadowExt struct {
		*spyExtender
		EnqueueChan int
	}
	me := &ShadowExt{
		newSpy(new(DefaultExtender), buf),
		0,
	}
	opts := NewOptions(me)
	opts.LogFlags = LogInfo
	c := NewCrawlerWithOptions(opts)

	c.Run("")

	assertIsInLog(tc.name, me.b, "extender.EnqueueChan is not of type chan<-interface{}, cannot set the enqueue channel\n", t)
}

func testEnqueueNewUrl(t *testing.T, tc *testCase, buf bool) {
	ff := newFileFetcher()
	spy := newSpy(ff, buf)
	spy.setExtensionMethod(eMKFilter, func(ctx *URLContext, isVisited bool) bool {
		// Accept only non-visited Page1s
		return !isVisited && strings.HasSuffix(strings.ToLower(ctx.url.Path), "page1.html")
	})
	enqueued := false
	spy.setExtensionMethod(eMKEnqueued, func(ctx *URLContext) {
		// Add hostc's Page1 to crawl
		if !enqueued {
			newU, err := url.Parse("http://hostc/page1.html")
			if err != nil {
				panic(err)
			}
			spy.EnqueueChan <- newU
			enqueued = true
		}
	})

	opts := NewOptions(spy)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	opts.SameHostOnly = false
	c := NewCrawlerWithOptions(opts)

	c.Run("http://hostb/page1.html")

	assertCallCount(spy, tc.name, eMKFilter, 7, t)
	assertCallCount(spy, tc.name, eMKEnqueued, 4, t) // robots.txt * 2, both Page1s
}

func testEnqueueNewUrlOnError(t *testing.T, tc *testCase, buf bool) {
	ff := newFileFetcher()
	spy := newSpy(ff, buf)
	spy.setExtensionMethod(eMKFilter, func(ctx *URLContext, isVisited bool) bool {
		// If is visited, but has a state of "Error", allow
		st, ok := ctx.State.(string)
		if isVisited && ok && st == "Error" {
			return true
		}
		// Accept only non-visited by default
		return !isVisited
	})
	once := false
	spy.setExtensionMethod(eMKError, func(err *CrawlError) {
		if err.Kind == CekFetch && !once {
			// On error, reenqueue once only
			once = true
			spy.EnqueueChan <- map[*url.URL]interface{}{
				err.Ctx.url: "Error",
			}
		}
	})

	opts := NewOptions(spy)
	opts.LogFlags = LogAll
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page6.html") // Page6 does not exist, that's the goal, generate an error

	assertCallCount(spy, tc.name, eMKFilter, 2, t)   // First pass and re-enqueued from error
	assertCallCount(spy, tc.name, eMKEnqueued, 3, t) // Twice and robots.txt
}

// TODO : Test to assert low CPU usage during long crawl delay waits? (issue #12)
