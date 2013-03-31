package gocrawl

import (
	"strings"
	"testing"
	"time"
)

// Type a is a simple syntax helper to create test cases' asserts.
type a map[extensionMethodKey]int64

// Type f is a simple syntax helper to create test cases' extension functions.
type f map[extensionMethodKey]interface{}

// Test case structure.
type testCase struct {
	name       string
	opts       *Options
	seeds      interface{}
	funcs      f
	asserts    a
	logAsserts []string
}

var (
	// Actual definition of test cases.
	// Prefix name with "*" to run this single starred test.
	// Prefix name with "!" to ignore this test.
	cases = [...]*testCase{
		&testCase{
			name: "AllSameHost",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page1.html",
				"http://hosta/page4.html",
			},
			asserts: a{
				eMKVisit:  5,
				eMKFilter: 13,
			},
		},

		&testCase{
			name: "AllNotSameHost",
			opts: &Options{
				SameHostOnly: false,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page1.html",
				"http://hosta/page4.html",
			},
			asserts: a{
				eMKVisit:  10,
				eMKFilter: 24,
			},
		},

		&testCase{
			name: "SelectOnlyPage1s",
			opts: &Options{
				SameHostOnly: false,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page1.html",
				"http://hosta/page4.html",
				"http://hostb/pageunlinked.html",
			},
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					if ctx.normalizedURL.Path == "/page1.html" {
						return !isVisited
					}
					return false
				},
			},
			asserts: a{
				eMKVisit:  3, // hostd not visited (not even fetched) because linked only from hostc/page3, which is not a page1
				eMKFilter: 11,
			},
		},

		&testCase{
			name: "IdleTimeOut",
			opts: &Options{
				SameHostOnly:  false,
				WorkerIdleTTL: 50 * time.Millisecond,
				CrawlDelay:    DefaultTestCrawlDelay,
				LogFlags:      LogInfo,
			},
			seeds: []string{
				"http://hosta/page1.html",
				"http://hosta/page4.html",
				"http://hostb/pageunlinked.html",
			},
			logAsserts: []string{
				"worker for host hostd cleared on idle policy\n",
				"worker for host hostunknown cleared on idle policy\n",
			},
		},
	}
)

func TestRunner(t *testing.T) {
	var singleTC *testCase

	// Check if a single case should run
	for _, tc := range cases {
		if strings.HasPrefix(tc.name, "*") {
			if singleTC == nil {
				singleTC = tc
			} else {
				t.Fatal("multiple test cases for isolated run (prefixed with '*')")
			}
		}
	}
	if singleTC != nil {
		singleTC.name = singleTC.name[1:]
		t.Logf("running %s in isolated run...", singleTC.name)
		runTestCase(t, singleTC)
	} else {
		for _, tc := range cases {
			if strings.HasPrefix(tc.name, "!") {
				t.Logf("ignoring %s", tc.name[1:])
			} else {
				t.Logf("running %s...", tc.name)
				runTestCase(t, tc)
			}
		}
	}
}

func runTestCase(t *testing.T, tc *testCase) {
	ff := newFileFetcher()
	spy := newSpy(ff, true)
	tc.opts.Extender = spy
	c := NewCrawlerWithOptions(tc.opts)
	if tc.funcs != nil {
		for emk, f := range tc.funcs {
			spy.setExtensionMethod(emk, f)
		}
	}

	if err := c.Run(tc.seeds); err != nil && err != ErrMaxVisits {
		t.Errorf("FAIL %s - %s.", tc.name, err)
	}

	assertCnt := 0
	for emk, cnt := range tc.asserts {
		assertCallCount(spy, tc.name, emk, cnt, t)
		assertCnt++
	}
	for _, s := range tc.logAsserts {
		if strings.HasPrefix(s, "!") {
			assertIsNotInLog(tc.name, spy.b, s[1:], t)
		} else {
			assertIsInLog(tc.name, spy.b, s, t)
		}
		assertCnt++
	}
	if assertCnt == 0 {
		t.Errorf("FAIL %s - no asserts.", tc.name)
	}
}

// TODO : Test Panic in visit, filter, etc.
// TODO : Test state with URL, various types supported as interface{} for seeds and harvested
/*
func TestIdleTimeOut(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = false
	opts.WorkerIdleTTL = 50 * time.Millisecond
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogInfo
	spy := runFileFetcherWithOptions(opts,
		[]string{"*"},
		[]string{"http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html"})

	assertIsInLog(spy.b, "worker for host hostd cleared on idle policy\n", t)
	assertIsInLog(spy.b, "worker for host hostunknown cleared on idle policy\n", t)
}

func TestReadBodyInVisitor(t *testing.T) {
	var err error
	var b []byte

	spy := newSpyExtenderFunc(eMKVisit, func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		b, err = ioutil.ReadAll(res.Body)
		return nil, false
	})

	c := NewCrawler(spy)
	c.Options.CrawlDelay = DefaultTestCrawlDelay
	c.Options.LogFlags = LogAll
	c.Run("http://hostc/page3.html")

	if err != nil {
		t.Error(err)
	} else if len(b) == 0 {
		t.Error("Empty body")
	}
}

func TestEnqueuedCount(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://robota/page1.html"})

	// page1 and robots.txt (did not visit page1, so page2 never found)
	assertCallCount(spy, eMKEnqueued, 2, t)
	// No visit per robots policy
	assertCallCount(spy, eMKVisit, 0, t)
}

func TestVisitedCount(t *testing.T) {
	opts := NewOptions(nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	spy := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html"})

	assertCallCount(spy, eMKVisited, 3, t)
}

func TestStartExtender(t *testing.T) {
	spy := newSpyExtenderFunc(eMKStart, func(seeds []string) []string {
		return append(seeds, "http://hostb/page1.html")
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostc/page1.html")

	assertCallCount(spy, eMKStart, 1, t)
	assertCallCount(spy, eMKVisit, 4, t)
	// Page1-2 for both, robots a-b, page unknown
	assertCallCount(spy, eMKEnqueued, 7, t)
}

func TestEndReasonMaxVisits(t *testing.T) {
	var e EndReason

	spy := newSpyExtenderFunc(eMKEnd, func(end EndReason) {
		e = end
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.MaxVisits = 1
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	assertCallCount(spy, eMKEnd, 1, t)
	if e != ErMaxVisits {
		t.Fatalf("Expected end reason MaxVisits, got %v\n", e)
	}
}

func TestEndReasonDone(t *testing.T) {
	var e EndReason

	spy := newSpyExtenderFunc(eMKEnd, func(end EndReason) {
		e = end
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page5.html")

	assertCallCount(spy, eMKEnd, 1, t)
	if e != ErDone {
		t.Fatalf("Expected end reason Done, got %v\n", e)
	}
}

func TestErrorFetch(t *testing.T) {
	var e *CrawlError

	spy := newSpyExtenderFunc(eMKError, func(err *CrawlError) {
		e = err
	})
	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostb/page1.html")

	assertCallCount(spy, eMKError, 1, t)
	if e.Kind != CekFetch {
		t.Fatalf("Expected error kind Fetch, got %v\n", e.Kind)
	}
}

func TestComputeDelay(t *testing.T) {
	spy := newSpyExtenderFunc(eMKComputeDelay, func(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
		return 17 * time.Millisecond
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogInfo
	opts.MaxVisits = 2
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	assertCallCount(spy, eMKComputeDelay, 3, t)
	assertIsInLog(spy.b, "using crawl-delay: 17ms\n", t)
}

func TestFilter(t *testing.T) {
	spy := newSpyExtenderFunc(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		return strings.HasSuffix(u.Path, "page1.html"), 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogError | LogIgnored
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostc/page1.html")

	assertCallCount(spy, eMKFilter, 3, t)
	assertCallCount(spy, eMKEnqueued, 2, t) // robots.txt triggers Enqueued too
	assertIsInLog(spy.b, "ignore on filter policy: http://hostc/page2.html\n", t)
}

func TestNoHead(t *testing.T) {
	var calledWithHead bool

	ff := newFileFetcher()

	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		if headRequest {
			calledWithHead = true
		}
		return ff.Fetch(u, userAgent, headRequest)
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = false
	opts.LogFlags = LogError | LogIgnored
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostb/page1.html")

	if calledWithHead {
		t.Fatal("Expected Fetch() to never be called with a HEAD request.")
	}
	assertCallCount(spy, eMKRequestGet, 0, t)
	assertCallCount(spy, eMKFetch, 4, t) // robots.txt and unknown.html triggers Fetch
}

func TestAllHead(t *testing.T) {
	var calledWithHead int
	var calledWithoutHead int

	ff := newFileFetcher()

	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		if headRequest {
			calledWithHead += 1
		} else {
			calledWithoutHead += 1
		}
		return ff.Fetch(u, userAgent, headRequest)
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = true
	opts.LogFlags = LogError | LogIgnored
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	if calledWithHead != (calledWithoutHead - 1) {
		t.Fatalf("Expected HEAD requests %d to be equal to GET requests %d minus one (for robots.txt).", calledWithHead, calledWithoutHead)
	}
	assertCallCount(spy, eMKFetch, 7, t) // Once for robots.txt, twice each for page1-3
	assertCallCount(spy, eMKRequestGet, 3, t)
	assertCallCount(spy, eMKEnqueued, 4, t)
}

func TestAllHeadWithFetchError(t *testing.T) {
	var calledWithHead int
	var calledWithoutHead int

	ff := newFileFetcher()

	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		if headRequest {
			calledWithHead += 1
		} else {
			calledWithoutHead += 1
		}
		if u.Path == "/unknown.html" {
			return nil, errors.New("Forced error")
		}
		return ff.Fetch(u, userAgent, headRequest)
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = true
	opts.LogFlags = LogError | LogIgnored
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostb/page1.html")

	// Head should be = 3 (page1, 2, unknown), Get should be = 3 (robots, page1, 2)
	if calledWithHead != (calledWithoutHead) {
		t.Fatalf("Expected HEAD requests %d to be equal to GET requests %d.", calledWithHead, calledWithoutHead)
	}
	assertCallCount(spy, eMKFetch, 6, t) // Once for robots.txt and unkwown.html, twice each for page1,2
	assertCallCount(spy, eMKRequestGet, 2, t)
	assertCallCount(spy, eMKEnqueued, 4, t)
}

func TestRequestGetFalse(t *testing.T) {
	var calledWithHead int
	var calledWithoutHead int

	ff := newFileFetcher()

	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		if headRequest {
			calledWithHead += 1
		} else {
			calledWithoutHead += 1
		}
		return ff.Fetch(u, userAgent, headRequest)
	})

	spy.setExtensionMethod(eMKRequestGet, func(headRes *http.Response) bool {
		if headRes.Request.URL.Path == "/page2.html" {
			return false
		}
		return true
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = true
	opts.LogFlags = LogError | LogIgnored
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	if calledWithHead != calledWithoutHead {
		// 3 GET: robots, page1, page3; 3 HEAD: page1, page2, page3
		t.Fatalf("Expected HEAD requests %d to be equal to GET requests %d.", calledWithHead, calledWithoutHead)
	}
	assertCallCount(spy, eMKFetch, 6, t) // Once for robots.txt and page2, twice each for page1 and page3
	assertCallCount(spy, eMKRequestGet, 3, t)
	assertCallCount(spy, eMKEnqueued, 4, t)
	assertCallCount(spy, eMKVisit, 2, t)
}

func TestHeadTrueFilterOverride(t *testing.T) {
	var calledWithHead int
	var calledWithoutHead int

	ff := newFileFetcher()
	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		if headRequest {
			calledWithHead += 1
		} else {
			calledWithoutHead += 1
		}
		return ff.Fetch(u, userAgent, headRequest)
	})

	// Page2: No get, Page3: No enqueue
	spy.setExtensionMethod(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, headRequest HeadRequestMode) {
		if u.Path == "/page2.html" {
			return !isVisited, 0, HrmIgnore
		} else if u.Path == "/page3.html" {
			return false, 0, HrmDefault
		}
		return !isVisited, 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = true
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	// 3 GET: robots, page1, page2; 1 HEAD: page1
	if calledWithHead != 1 {
		t.Fatalf("Expected 1 HEAD requests, got %d.", calledWithHead)
	}
	if calledWithoutHead != 3 {
		t.Fatalf("Expected 3 GET requests, got %d.", calledWithoutHead)
	}
	assertCallCount(spy, eMKFetch, 4, t)      // Once for robots.txt and page2, twice each for page1
	assertCallCount(spy, eMKRequestGet, 1, t) // Page1 only, page2 ignored HEAD
	assertCallCount(spy, eMKEnqueued, 3, t)   // Page1-2 and robots
}

func TestHeadFalseFilterOverride(t *testing.T) {
	var calledWithHead int
	var calledWithoutHead int

	ff := newFileFetcher()
	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		if headRequest {
			calledWithHead += 1
		} else {
			calledWithoutHead += 1
		}
		return ff.Fetch(u, userAgent, headRequest)
	})

	// Page1: default, Page2: Head before get, Page3: No enqueue
	spy.setExtensionMethod(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, headRequest HeadRequestMode) {
		if u.Path == "/page2.html" {
			return !isVisited, 0, HrmRequest
		} else if u.Path == "/page3.html" {
			return false, 0, HrmDefault
		}
		return !isVisited, 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = false
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html")

	// 3 GET: robots, page1, page2; 1 HEAD: page2
	if calledWithHead != 1 {
		t.Fatalf("Expected 1 HEAD requests, got %d.", calledWithHead)
	}
	if calledWithoutHead != 3 {
		t.Fatalf("Expected 3 GET requests, got %d.", calledWithoutHead)
	}
	assertCallCount(spy, eMKFetch, 4, t)      // Once for robots.txt and page1, twice for page2
	assertCallCount(spy, eMKRequestGet, 1, t) // Page2 only, page1 ignored HEAD
	assertCallCount(spy, eMKEnqueued, 3, t)   // Page1-2 and robots
}

func TestHeadResponse(t *testing.T) {
	var b []byte
	var e error
	var headLen int
	de := new(DefaultExtender)

	spy := newSpyExtenderFunc(eMKRequestGet, func(headRes *http.Response) bool {
		headLen = len(headRes.Header)
		b, e = ioutil.ReadAll(headRes.Body)
		return false
	})
	spy.setExtensionMethod(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		return de.Fetch(u, userAgent, headRequest)
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.HeadBeforeGet = true
	opts.LogFlags = LogAll
	opts.MaxVisits = 1
	c := NewCrawlerWithOptions(opts)
	c.Run("http://provok.in")

	if e != nil {
		t.Fatal(e)
	}
	if len(b) > 0 {
		t.Fatal("Expected body to be empty.")
	}
	if headLen == 0 {
		t.Fatal("Expected headers to be present.")
	}
}

func TestCrawlDelay(t *testing.T) {
	var last time.Time
	var since []time.Duration
	cnt := 0

	ff := newFileFetcher()
	spy := newSpyExtenderFunc(eMKFetch, func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
		since = append(since, time.Now().Sub(last))
		last = time.Now()
		return ff.Fetch(u, userAgent, headRequest)
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

	assertCallCount(spy, eMKFetch, 7, t)
	assertCallCount(spy, eMKComputeDelay, 7, t)
	for i, d := range since {
		min := (DefaultTestCrawlDelay * time.Duration(i))
		t.Logf("Actual delay for request %d is %v.", i, d)
		if d < min {
			t.Errorf("Expected a delay of at least %v for fetch #%d, got %v.", min, i, d)
		}
	}
}

func TestUserAgent(t *testing.T) {
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
		if r.UserAgent() != c.Options.RobotUserAgent {
			t.Errorf("Expected user-agent %s, got %s", c.Options.RobotUserAgent, r.UserAgent())
		}
	})
	http.HandleFunc("/bidon", func(w http.ResponseWriter, r *http.Request) {
		// Expect crawl user agent
		if r.UserAgent() != c.Options.UserAgent {
			t.Errorf("Expected user-agent %s, got %s", c.Options.UserAgent, r.UserAgent())
		}
	})

	// Start server in a separate goroutine
	go func() {
		http.Serve(l, nil)
	}()

	// Go crawl
	c.Run("http://localhost:8080/bidon")

	// Close listener
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
}

// Test issue #10
func TestRedirectRelative(t *testing.T) {
	s := newSpy(new(DefaultExtender), true)
	c := NewCrawler(s)
	c.Options.LogFlags = LogAll
	c.Run("http://golang.org/pkg") // Will redirect to /pkg/, then normalized to /pkg
	str := "ignore on absolute policy: /pkg"
	assertIsNotInLog(s.b, str, t)
}

// Test issue #10
func TestCircularRedirect(t *testing.T) {
	var cnt int

	s := newSpy(new(DefaultExtender), true)
	s.setExtensionMethod(eMKFilter, func(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode) {
		// Always enqueue if /pkg, ignore all others
		if target.Path == "/pkg" {
			cnt++
			if cnt <= 2 {
				// Should filter only twice, /pkg and /pkg/, otherwise, circular problem
				return true, 0, HrmDefault
			}
		}
		return false, 0, HrmDefault
	})

	c := NewCrawler(s)
	c.Options.MaxVisits = 1
	c.Options.LogFlags = LogAll
	c.Run("http://golang.org/pkg") // Will redirect to /pkg/, then normalized to /pkg, hence the problematic loop
	if cnt > 2 {
		t.Errorf("Expected only 2 calls to Filter, one for /pkg and one for /pkg/, got %d", cnt)
	}
	assertCallCount(s, eMKEnqueued, 3, t) // Expect 3 Enqueued calls: robots, /pkg (redirects), /pkg/
	assertCallCount(s, eMKVisited, 1, t)  // Expect 1 visit : /pkg/ (robots don't trigger visited)
}

// Test issue #13
func TestSameHostPolicyWithNormalizedSourceUrl(t *testing.T) {
	spy := newSpyExtenderFunc(eMKVisit, func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		if res.Request.URL.Path == "/page1.html" {
			u, err := res.Request.URL.Parse("page2.html")
			if err != nil {
				panic(err)
			}
			return []*url.URL{u}, false
		}
		return nil, false
	})

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	c.Run("http://www.hosta/page1.html")

	// Robots don't trigger Filter nor Visit
	assertCallCount(spy, eMKFilter, 2, t)     // page1, page2
	assertCallCount(spy, eMKVisit, 2, t)      // page1, page2
	assertCallCount(spy, eMKDisallowed, 0, t) // No disallowed page
}

// Test issue #13
func TestSameHostPolicyRejectWithNormalizedSourceUrl(t *testing.T) {
	spy := newSpyExtenderFunc(eMKVisit, func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		if res.Request.URL.Host == "www.hosta" {
			u, err := url.Parse("http://www.hostb/page1.html")
			if err != nil {
				panic(err)
			}
			return []*url.URL{u}, false
		}
		return nil, false
	})
	spy.useLogBuffer = true

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	c.Run("http://www.hosta/page1.html")

	// Robots don't trigger Filter nor Visit
	assertCallCount(spy, eMKFilter, 2, t) // hosta/page1, hostb/page1
	assertCallCount(spy, eMKVisit, 1, t)  // hosta/page1
	str := "ignore on same host policy: http://hostb/page1.html"
	assertIsInLog(spy.b, str, t)
}
func TestRunTwiceSameInstance(t *testing.T) {
	spy := newSpyExtenderConfigured(0, nil, true, 0, "*")

	opts := NewOptions(spy)
	opts.SameHostOnly = true
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogNone
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page1.html", "http://hosta/page4.html")

	assertCallCount(spy, eMKVisit, 5, t)
	assertCallCount(spy, eMKFilter, 13, t)

	spy = newSpyExtenderConfigured(0, nil, true, 0, "http://hosta/page1.html", "http://hostb/page1.html", "http://hostc/page1.html", "http://hostd/page1.html")
	opts.SameHostOnly = false
	opts.Extender = spy
	c.Run("http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html")

	assertCallCount(spy, eMKVisit, 3, t)
	assertCallCount(spy, eMKFilter, 11, t)
}
*/
