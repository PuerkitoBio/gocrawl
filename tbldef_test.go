package gocrawl

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
)

// Type a is a simple syntax helper to create test cases' asserts.
type a map[extensionMethodKey]int

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
	panics       bool
	external     func(*testing.T, *testCase, bool)
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
			customAssert: func(spy *spyExtender, t *testing.T) {
				v := runtime.Version()
				if strings.HasPrefix(v, "go1.0") {
					assertCallCount(spy, "InvalidSeed", eMKError, 1, t)
					assertIsInLog("InvalidSeed", spy.b, "ERROR parsing URL #toto\n", t)
				} else {
					assertIsInLog("InvalidSeed", spy.b, "ignore on absolute policy: #toto\n", t)
				}
				assertCallCount(spy, "InvalidSeed", eMKVisit, 0, t)
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
					if strings.ToLower(headRes.Request.URL.Path) == "/page2.html" {
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
				head := s.getCalledWithCount(eMKFetch, __, __, true)
				nohead := s.getCalledWithCount(eMKFetch, __, __, false)
				// 3 GET: robots, page1, page3; 3 HEAD: page1, page2, page3
				assertTrue(head == 3, "expected 3 HEAD requests, got %d", head)
				assertTrue(nohead == 3, "expected 3 GET requests, got %d", nohead)
			},
		},

		&testCase{
			name: "NoHead",
			opts: &Options{
				SameHostOnly:  true,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: false,
				LogFlags:      LogAll,
			},
			seeds: []string{
				"http://hostb/page1.html",
			},
			asserts: a{
				eMKFetch:      4, // robots.txt and unknown.html triggers Fetch
				eMKRequestGet: 0,
			},
			customAssert: func(s *spyExtender, t *testing.T) {
				head := s.getCalledWithCount(eMKFetch, __, __, true)
				assertTrue(head == 0, "expected no HEAD request, got %d", head)
			},
		},

		&testCase{
			name: "AllHead",
			opts: &Options{
				SameHostOnly:  true,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: true,
				LogFlags:      LogAll,
			},
			seeds: "http://hosta/page1.html",
			asserts: a{
				eMKFetch:      7, // Once for robots.txt, twice each for page1-3
				eMKRequestGet: 3,
				eMKEnqueued:   4,
			},
			customAssert: func(s *spyExtender, t *testing.T) {
				head := s.getCalledWithCount(eMKFetch, __, __, true)
				nohead := s.getCalledWithCount(eMKFetch, __, __, false)
				assertTrue(head == nohead-1, "expected HEAD requests to be equal to GET requests minus one (robots.txt)")
			},
		},

		&testCase{
			name: "AllHeadWithFetchError",
			opts: &Options{
				SameHostOnly:  true,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: true,
				LogFlags:      LogAll,
			},
			seeds: "http://hostb/page1.html",
			asserts: a{
				eMKFetch:      6, // Once for robots.txt and unkwown.html, twice each for page1,2
				eMKRequestGet: 2,
				eMKEnqueued:   4,
				eMKError:      1, // unknown.html HEAD request
			},
			customAssert: func(s *spyExtender, t *testing.T) {
				head := s.getCalledWithCount(eMKFetch, __, __, true)
				nohead := s.getCalledWithCount(eMKFetch, __, __, false)
				// Head should be = 3 (page1, 2, unknown), Get should be = 3 (robots, page1, 2)
				assertTrue(head == 3, "expected 3 HEAD requests, got %d", head)
				assertTrue(nohead == 3, "expected 3 GET requests, got %d", nohead)
			},
		},

		&testCase{
			name: "HeadTrueOverride",
			opts: &Options{
				SameHostOnly:  true,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: true,
				LogFlags:      LogAll,
			},
			seeds: "http://hosta/page1.html",
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					// Page2: No HEAD, Page3: No enqueue
					if strings.ToLower(ctx.url.Path) == "/page2.html" {
						ctx.HeadBeforeGet = false
						return !isVisited
					} else if strings.ToLower(ctx.url.Path) == "/page3.html" {
						return false
					}
					return !isVisited
				},
			},
			asserts: a{
				eMKFetch:      4, // Once for robots.txt and page2, twice for page1
				eMKRequestGet: 1, // Page1 only, page2 ignored HEAD
				eMKEnqueued:   3, // Page1-2 and robots
			},
			customAssert: func(s *spyExtender, t *testing.T) {
				head := s.getCalledWithCount(eMKFetch, __, __, true)
				nohead := s.getCalledWithCount(eMKFetch, __, __, false)
				// 3 GET: robots, page1, page2; 1 HEAD: page1
				assertTrue(head == 1, "expected 1 HEAD request, got %d", head)
				assertTrue(nohead == 3, "expected 3 GET requests, got %d", nohead)
			},
		},

		&testCase{
			name: "HeadFalseOverride",
			opts: &Options{
				SameHostOnly:  true,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: false,
				LogFlags:      LogAll,
			},
			seeds: "http://hosta/page1.html",
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					// Page1: default, Page2: Head before get, Page3: No enqueue
					if strings.ToLower(ctx.url.Path) == "/page2.html" {
						ctx.HeadBeforeGet = true
						return !isVisited
					} else if strings.ToLower(ctx.url.Path) == "/page3.html" {
						return false
					}
					return !isVisited
				},
			},
			asserts: a{
				eMKFetch:      4, // Once for robots.txt and page1, twice for page2
				eMKRequestGet: 1, // Page2 only, page1 ignored HEAD
				eMKEnqueued:   3, // Page1-2 and robots
			},
			customAssert: func(s *spyExtender, t *testing.T) {
				head := s.getCalledWithCount(eMKFetch, __, __, true)
				nohead := s.getCalledWithCount(eMKFetch, __, __, false)
				// 3 GET: robots, page1, page2; 1 HEAD: page2
				assertTrue(head == 1, "expected 1 HEAD request, got %d", head)
				assertTrue(nohead == 3, "expected 3 GET requests, got %d", nohead)
			},
		},

		&testCase{
			name: "RedirectFilterOut",
			http: true,
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "http://src.ca",
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					// Accept only src.ca
					return !isVisited && ctx.url.Host == "src.ca"
				},
			},
			asserts: a{
				eMKFilter:   2, // src.ca and radio-canada.ca
				eMKEnqueued: 2, // src.ca and robots.txt
				eMKFetch:    2, // src.ca and robots.txt
				eMKVisit:    0, // src.ca redirects, so no visit
			},
		},

		// ignore this test, now src.ca redirects to radio-canada.ca, then to ici.radio-canada.ca
		// too fragile.
		&testCase{
			name: "!RedirectFollow",
			http: true,
			opts: &Options{
				SameHostOnly: false,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "http://src.ca",
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					return !isVisited && ctx.sourceURL == nil
				},
			},
			asserts: a{
				eMKEnqueued: 4, // src.ca, radio-canada.ca and both robots.txt
				eMKFetch:    4, // src.ca, radio-canada.ca and both robots.txt
				eMKVisit:    1, // src.ca redirects, radio-canada.ca visited
			},
		},

		// Like RedirectFollow, src.ca has changed and redirects more times.
		// Brittle test case.
		&testCase{
			name: "!RedirectFollowHeadFirst",
			http: true,
			opts: &Options{
				SameHostOnly:  false,
				CrawlDelay:    DefaultTestCrawlDelay,
				HeadBeforeGet: true,
				LogFlags:      LogAll,
			},
			seeds: "http://src.ca",
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					return !isVisited && ctx.sourceURL == nil
				},
			},
			asserts: a{
				eMKEnqueued:   4, // src.ca, radio-canada.ca and both robots.txt
				eMKRequestGet: 1, // radio-canada.ca only (no HEAD for robots, and src.ca gets redirected)
				eMKFetch:      5, // src.ca, 2*radio-canada.ca and both robots.txt
				eMKVisit:      1, // src.ca redirects, radio-canada.ca visited
			},
		},

		&testCase{
			name: "PanicInFilter",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "http://hosta/page1.html",
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					panic("error")
				},
			},
			panics: true,
		},

		&testCase{
			name: "VisitReturnsURLsWithStateUsingS",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "http://hosta/page1.html",
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					if ctx.sourceURL == nil {
						// Only when called for seed
						return S{
							"http://hosta/page2.html": 2,
							"http://hosta/page3.html": 3,
							"http://hosta/page4.html": 4,
							"http://hosta/page5.html": 5,
						}, false
					} else {
						rx := regexp.MustCompile(`/page(\d)\.html`)
						mtch := rx.FindStringSubmatch(ctx.normalizedURL.Path)
						i, ok := ctx.State.(int)
						if assertTrue(ok, "expected state data to be an int for %s", ctx.normalizedURL) {
							if page, err := strconv.Atoi(mtch[1]); err != nil {
								panic(err)
							} else {
								assertTrue(page == i, "expected state for page%d.html to be %d, got %d", page, page, i)
							}
						}
					}
					return nil, false
				},
			},
			asserts: a{
				eMKFilter:   5,
				eMKVisit:    5,
				eMKEnqueued: 6, // 5 pages + robots
			},
		},

		&testCase{
			name: "VisitReturnsURLsWithStateUsingU",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds: "http://hosta/page1.html",
			funcs: f{
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					if ctx.sourceURL == nil {
						// Only when called for seed
						res := make(U, 4)
						for i := 2; i <= 5; i++ {
							u, e := url.Parse(fmt.Sprintf("http://hosta/page%d.html", i))
							if e != nil {
								panic(e)
							}
							res[u] = i
						}
						return res, false
					} else {
						rx := regexp.MustCompile(`/page(\d)\.html`)
						mtch := rx.FindStringSubmatch(ctx.normalizedURL.Path)
						i, ok := ctx.State.(int)
						if assertTrue(ok, "expected state data to be an int for %s", ctx.normalizedURL) {
							if page, err := strconv.Atoi(mtch[1]); err != nil {
								panic(err)
							} else {
								assertTrue(page == i, "expected state for page%d.html to be %d, got %d", page, page, i)
							}
						}
					}
					return nil, false
				},
			},
			asserts: a{
				eMKFilter:   5,
				eMKVisit:    5,
				eMKEnqueued: 6, // 5 pages + robots
			},
		},

		&testCase{
			name: "PanicIfInvalidSeedType",
			opts: &Options{
				SameHostOnly: true,
				CrawlDelay:   DefaultTestCrawlDelay,
				LogFlags:     LogAll,
			},
			seeds:  212,
			panics: true,
		},

		&testCase{
			name: "QueryStringLostAfterNormalization-i16",
			http: true,
			opts: &Options{
				SameHostOnly:          false,
				CrawlDelay:            DefaultTestCrawlDelay,
				LogFlags:              LogAll,
				URLNormalizationFlags: purell.FlagsUsuallySafeNonGreedy,
			},
			seeds: []string{
				"http://www.example.com/",
			},
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					return !isVisited
				},
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					return "http://www.example.com/new/?start=60", false
				},
			},
			logAsserts: []string{
				"enqueue: http://www.example.com/new/?start=60\n",
			},
		},

		&testCase{
			name: "QueryStringLostAfterNormalizationWithParse-i16",
			http: true,
			opts: &Options{
				SameHostOnly:          false,
				CrawlDelay:            DefaultTestCrawlDelay,
				LogFlags:              LogAll,
				URLNormalizationFlags: purell.FlagsUsuallySafeNonGreedy,
			},
			seeds: []string{
				"http://www.example.com/",
			},
			funcs: f{
				eMKFilter: func(ctx *URLContext, isVisited bool) bool {
					return !isVisited
				},
				eMKVisit: func(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
					u, err := url.Parse("http://www.example.com/new/?start=60")
					if err != nil {
						panic(err)
					}
					return u, false
				},
			},
			logAsserts: []string{
				"enqueue: http://www.example.com/new/?start=60\n",
			},
		},

		&testCase{
			name:     "NoCrawlDelay",
			external: testNoCrawlDelay,
		},

		&testCase{
			name:     "NoExtender",
			external: testNoExtender,
		},

		&testCase{
			name:     "CrawlDelay",
			external: testCrawlDelay,
		},

		&testCase{
			name:     "UserAgent",
			external: testUserAgent,
		},

		&testCase{
			name:     "RunTwiceSameInstance",
			external: testRunTwiceSameInstance,
		},

		&testCase{
			name:     "EnqueueChanEmbedded",
			external: testEnqueueChanEmbedded,
		},

		&testCase{
			name:     "EnqueueChanShadowed",
			external: testEnqueueChanShadowed,
		},

		&testCase{
			name:     "EnqueueNewUrl",
			external: testEnqueueNewUrl,
		},

		&testCase{
			name:     "EnqueueNewUrlOnError",
			external: testEnqueueNewUrlOnError,
		},
	}
)
