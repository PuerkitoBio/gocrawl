package gocrawl

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"net/http"
	"net/url"
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
	name         string
	opts         *Options
	http         bool
	seeds        interface{}
	funcs        f
	asserts      a
	logAsserts   []string
	customAssert func(*spyExtender, *testing.T)
}

var (
	// Global variable for the in-context of the test case generic assert function.
	assertTrue func(bool, string, ...interface{}) bool

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

		&testCase{
			name: "EnqueuedCount",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://robota/page1.html",
			},
			asserts: a{
				eMKEnqueued: 2, // page1 and robots.txt (did not visit page1, so page2 never found)
				eMKVisit:    0, // No visit per robots policy
			},
		},

		&testCase{
			name: "VisitedCount",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page1.html",
			},
			asserts: a{
				eMKVisited: 3,
			},
		},

		&testCase{
			name: "StartExtender",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hostc/page1.html",
			},
			funcs: f{
				eMKStart: func(seeds interface{}) interface{} {
					ar := seeds.([]string)
					return append(ar, "http://hostb/page1.html")
				},
			},
			asserts: a{
				eMKStart:    1,
				eMKVisit:    4,
				eMKEnqueued: 7, // Page1-2 for both, robots a-b, page unknown
			},
		},

		&testCase{
			name: "!EndErrMaxVisits",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				MaxVisits:    1,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page1.html",
			},
			// TODO : Implement a "called with args" mock-type feature
			asserts: a{
				eMKStart:    1,
				eMKVisit:    4,
				eMKEnqueued: 7, // Page1-2 for both, robots a-b, page unknown
			},
		},

		&testCase{
			name: "ComputeDelay",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogError | LogInfo,
			},
			seeds: []string{
				"http://hosta/page1.html",
			},
			funcs: f{
				eMKComputeDelay: func(host string, di *DelayInfo, lf *FetchInfo) time.Duration {
					return 17 * time.Millisecond
				},
			},
			asserts: a{
				eMKComputeDelay: 4,
			},
			logAsserts: []string{
				"using crawl-delay: 17ms\n",
			},
		},

		&testCase{
			name: "Filter",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogError | LogIgnored,
			},
			seeds: []string{
				"http://hostc/page1.html",
			},
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					return strings.HasSuffix(ctx.url.Path, "page1.html")
				},
			},
			asserts: a{
				eMKFilter:   3,
				eMKEnqueued: 2, // robots.txt triggers Enqueued too
			},
			logAsserts: []string{
				"ignore on filter policy: http://hostc/page2.html\n",
			},
		},

		&testCase{
			name: "HeadResponse",
			http: true,
			opts: &Options{
				SameHostOnly:  true,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: true,
				MaxVisits:     1,
				LogFlags:      LogAll,
			},
			seeds: []string{
				"http://httpstat.us/200",
			},
			funcs: f{
				eMKRequestGet: func(ctx *URLContext, headRes *http.Response) bool {
					assertTrue(len(headRes.Header) > 0, "expected headers to be present")
					b, e := ioutil.ReadAll(headRes.Body)
					if assertTrue(e == nil, "%s", e) {
						assertTrue(len(b) == 0, "expected body to be empty")
					}
					return false
				},
			},
		},

		&testCase{
			name: "RedirectRelative-i10",
			http: true,
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://golang.org/pkg",
			},
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (harvested interface{}, findLinks bool) {
					return nil, false
				},
			},
			logAsserts: []string{
				"!ignore on absolute policy: /pkg",
			},
		},

		&testCase{
			name: "CircularRedirect-i10",
			http: true,
			opts: &Options{
				SameHostOnly: true,
				MaxVisits:    1,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://golang.org/pkg",
			},
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (harvested interface{}, findLinks bool) {
					return nil, false
				},
			},
			asserts: a{
				eMKEnqueued: 3, // Expect 3 Enqueued calls: robots, /pkg (redirects), /pkg/
				eMKFilter:   2, // One for /pkg and one for /pkg/
				eMKVisit:    1, // Expect 1 visit : /pkg/ (robots don't trigger visited)
			},
		},

		&testCase{
			name: "SameHostPolicyWithNormalizedSourceUrl-i13",
			opts: &Options{
				SameHostOnly:          true,
				CrawlDelay:            DefaultTestCrawlDelay,
				URLNormalizationFlags: DefaultNormalizationFlags,
				LogFlags:              LogAll,
			},
			seeds: []string{
				"http://www.hosta/page1.html",
			},
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					if res.Request.URL.Path == "/page1.html" {
						u, err := res.Request.URL.Parse("page2.html")
						if err != nil {
							panic(err)
						}
						return []*url.URL{u}, false
					}
					return nil, false
				},
			},
			asserts: a{
				eMKDisallowed: 0,
				eMKFilter:     2, // page1, page2
				eMKVisit:      2, // page1, page2
			},
		},

		&testCase{
			name: "SameHostPolicyRejectWithNormalizedSourceUrl-i13",
			opts: &Options{
				SameHostOnly:          true,
				CrawlDelay:            DefaultTestCrawlDelay,
				LogFlags:              LogAll,
				URLNormalizationFlags: DefaultNormalizationFlags,
			},
			seeds: []string{
				"http://www.hosta/page1.html",
			},
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					if res.Request.URL.Host == "www.hosta" {
						u, err := url.Parse("http://www.hostb/page1.html")
						if err != nil {
							panic(err)
						}
						return u, false
					}
					return nil, false
				},
			},
			asserts: a{
				eMKFilter: 2, // hosta/page1, hostb/page1
				eMKVisit:  1, // hosta/page1
			},
			logAsserts: []string{
				"ignore on same host policy: http://hostb/page1.html",
			},
		},

		&testCase{
			name: "ReadBodyInVisitor",
			opts: &Options{
				SameHostOnly:          true,
				CrawlDelay:            DefaultTestCrawlDelay,
				MaxVisits:             1,
				LogFlags:              LogAll,
				URLNormalizationFlags: DefaultNormalizationFlags,
			},
			seeds: []string{
				"http://hostc/page3.html",
			},
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					b, err := ioutil.ReadAll(res.Body)
					if assertTrue(err == nil, "%s", err) {
						assertTrue(len(b) > 0, "expected some content in the body")
					}
					return nil, false
				},
			},
		},

		&testCase{
			name: "EndReasonMaxVisits",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				MaxVisits:    1,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page1.html",
			},
			funcs: f{
				eMKEnd: func(err error) {
					assertTrue(err == ErrMaxVisits, "expected error to be ErrMaxVisits")
				},
			},
			asserts: a{
				eMKEnd: 1,
			},
		},

		&testCase{
			name: "EndReasonDone",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hosta/page5.html",
			},
			funcs: f{
				eMKEnd: func(err error) {
					assertTrue(err == nil, "expected error to be nil")
				},
			},
			asserts: a{
				eMKEnd: 1,
			},
		},

		&testCase{
			name: "ErrorFetch",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://hostb/page2.html", // Will try to load pageunknown.html
			},
			funcs: f{
				eMKError: func(err *CrawlError) {
					assertTrue(err.Kind == CekFetch, "expected error to be of kind %s, got %s", CekFetch, err.Kind)
				},
			},
			asserts: a{
				eMKError: 1,
			},
		},

		&testCase{
			name: "NonHtmlRequest",
			http: true,
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"https://lh4.googleusercontent.com/-v0soe-ievYE/AAAAAAAAAAI/AAAAAAAAs7Y/_UbxpxC-VG0/photo.jpg",
			},
			asserts: a{
				eMKError: 0,
				eMKVisit: 1,
			},
		},

		&testCase{
			name: "InvalidSeed",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"#toto",
			},
			asserts: a{
				eMKError: 1,
				eMKVisit: 0,
			},
			logAsserts: []string{
				"ERROR parsing URL #toto\n",
			},
		},

		&testCase{
			name: "HostCount",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"ftp://roota/a", // Use FTP scheme so that it doesn't actually attempt a fetch
				"ftp://roota/b",
				"ftp://rootb/c",
			},
			asserts: a{
				eMKVisit: 0,
			},
			logAsserts: []string{
				"init() - host count: 2\n",
				"init() - seeds length: 3\n",
			},
		},

		&testCase{
			name: "CustomFilterNoURL",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: []string{
				"http://test1",
				"http://test2",
			},
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					return false
				},
			},
			asserts: a{
				eMKVisit:  0,
				eMKFilter: 2,
			},
			logAsserts: []string{
				"ignore on filter policy: http://test1\n",
				"ignore on filter policy: http://test2\n",
			},
		},

		&testCase{
			name: "NoSeed",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: nil,
			asserts: a{
				eMKVisit:  0,
				eMKFilter: 0,
				eMKError:  0,
			},
		},

		&testCase{
			name: "NoVisitorFunc",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "http://hosta/page1.html",
			asserts: a{
				eMKVisit:  3, // With default Filter and Visit, will visit all same-host links
				eMKFilter: 10,
				eMKError:  0,
			},
		},

		&testCase{
			name: "EnqueueChanDefault",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "",
			customAssert: func(spy *spyExtender, t *testing.T) {
				assertTrue(spy.EnqueueChan != nil, "expected EnqueueChan to be non-nil")
			},
		},

		&testCase{
			name: "RobotDenyAll",
			opts: &Options{
				SameHostOnly:   false,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: DefaultRobotUserAgent,
			},
			seeds: "http://robota/page1.html",
			asserts: a{
				eMKVisit:  0,
				eMKFilter: 1,
			},
		},

		&testCase{
			name: "RobotPartialDenyGooglebot",
			opts: &Options{
				SameHostOnly:   false,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: DefaultRobotUserAgent,
			},
			seeds: "http://robotb/page1.html",
			asserts: a{
				eMKVisit:  2,
				eMKFilter: 4,
			},
		},

		&testCase{
			name: "RobotDenyOtherBot",
			opts: &Options{
				SameHostOnly:   false,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: "NotGoogleBot",
			},
			seeds: "http://robotb/page1.html",
			asserts: a{
				eMKVisit:  4,
				eMKFilter: 5,
			},
		},

		&testCase{
			name: "RobotExplicitAllowPattern",
			opts: &Options{
				SameHostOnly:   false,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: DefaultRobotUserAgent,
			},
			seeds: "http://robotc/page1.html",
			asserts: a{
				eMKVisit:  4,
				eMKFilter: 5,
			},
		},

		&testCase{
			name: "RobotCrawlDelay",
			opts: &Options{
				SameHostOnly:   true,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: DefaultRobotUserAgent,
			},
			seeds: "http://robotc/page1.html",
			asserts: a{
				eMKVisit:  4,
				eMKFilter: 5,
			},
			logAsserts: []string{
				"using crawl-delay: 200ms\n",
			},
		},

		&testCase{
			name: "CachedRobot",
			opts: &Options{
				SameHostOnly:   true,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: DefaultRobotUserAgent,
			},
			seeds: "http://robota/page1.html",
			funcs: f{
				eMKRequestRobots: func(ctx *URLContext, agent string) ([]byte, bool) {
					return []byte("User-agent: *\nDisallow:/page2.html"), false
				},
			},
			asserts: a{
				eMKVisit:         1,
				eMKEnqueued:      3,
				eMKRequestRobots: 1,
				eMKDisallowed:    1,
			},
		},

		&testCase{
			name: "FetchedRobot",
			opts: &Options{
				SameHostOnly:   true,
				CrawlDelay:     DefaultTestCrawlDelay,
				LogFlags:       LogAll,
				RobotUserAgent: DefaultRobotUserAgent,
			},
			seeds: "http://robotc/page4.html",
			funcs: f{
				eMKFetchedRobots: func(ctx *URLContext, res *http.Response) {
					b, err := ioutil.ReadAll(res.Body)
					if assertTrue(err == nil, "%s", err) {
						assertTrue(len(b) > 0, "expected fetched robots.txt body not to be empty")
					}
				},
			},
			asserts: a{
				eMKRequestRobots: 1,
				eMKEnqueued:      2,
				eMKFetchedRobots: 1,
			},
		},

		/*
			&testCase{
				name: "RequestGetFalse",
				opts: &Options{
					SameHostOnly:  true,
					CrawlDelay:    DefaultTestCrawlDelay,
					HeadBeforeGet: true,
					LogFlags:      LogAll,
				},
				seeds: []string{
					"http://hosta/page1.html",
				},
				funcs: f{
					eMKRequestGet: func(ctx *URLContext, headRes *http.Response) bool {
						if headRes.Request.URL.Path == "/page2.html" {
							return false
						}
						return true
					},
				},
				asserts: a{
					eMKFetch:      6, // Once for robots.txt and page2, twice each for page1 and page3
					eMKRequestGet: 3,
					eMKEnqueued:   4,
					eMKVisit:      2,
				},
				customAssert: func(s *spyExtender, t *testing.T) {
					head := s.calledWithCount(ignoreCalledArg, ignoreCalledArg, true)
					nohead := s.calledWithCount(ignoreCalledArg, ignoreCalledArg, false)
					if head != nohead {
						t.Errorf()
					}
				},
			},
		*/
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
	run := 0
	ign := 0
	if singleTC != nil {
		singleTC.name = singleTC.name[1:]
		run++
		t.Logf("running %s in isolated run...", singleTC.name)
		runTestCase(t, singleTC)
	} else {
		for _, tc := range cases {
			if strings.HasPrefix(tc.name, "!") {
				ign++
				t.Logf("ignoring %s", tc.name[1:])
			} else {
				run++
				t.Logf("running %s...", tc.name)
				runTestCase(t, tc)
			}
		}
	}
	t.Logf("%d test(s) executed, %d test(s) ignored", run, ign)
}

func runTestCase(t *testing.T, tc *testCase) {
	var spy *spyExtender

	// Setup the global assertTrue variable to a closure on this T and test case.
	assertCnt := 0
	assertTrue = func(cond bool, msg string, args ...interface{}) bool {
		assertCnt++
		if !cond {
			t.Errorf("FAIL %s - %s.", tc.name, fmt.Sprintf(msg, args...))
			return false
		}
		return true
	}

	if tc.http {
		ext := new(DefaultExtender)
		spy = newSpy(ext, true)
	} else {
		ff := newFileFetcher()
		spy = newSpy(ff, true)
	}
	if strings.HasSuffix(tc.name, ">") {
		// Debug mode, print log to screen instead of buffer, and log all
		spy.useLogBuffer = false
		tc.opts.LogFlags = LogAll
	}
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
	if tc.customAssert != nil {
		tc.customAssert(spy, t)
		assertCnt++
	}
	if assertCnt == 0 {
		t.Errorf("FAIL %s - no asserts.", tc.name)
	}
}

// TODO : Test Panic in visit, filter, etc.
// TODO : Test state with URL, various types supported as interface{} for seeds and harvested
/*
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
