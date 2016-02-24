package gocrawl

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"path"

	"github.com/PuerkitoBio/goquery"
	robotstxt "github.com/temoto/robotstxt.go"
	"golang.org/x/net/html"
)

// The worker is dedicated to fetching and visiting a given host, respecting
// this host's robots.txt crawling policies.
type worker struct {
	// Worker identification
	host  string
	index int

	// Communication channels and sync
	push    chan<- *workerResponse
	pop     popChannel
	stop    chan struct{}
	enqueue chan<- interface{}
	wg      *sync.WaitGroup

	// Robots validation
	robotsGroup *robotstxt.Group

	// Logging
	logFunc func(LogFlags, string, ...interface{})

	// Implementation fields
	wait           <-chan time.Time
	lastFetch      *FetchInfo
	lastCrawlDelay time.Duration
	opts           *Options
}

// Start crawling the host.
func (w *worker) run() {
	defer func() {
		w.logFunc(LogInfo, "worker done.")
		w.wg.Done()
	}()

	// Enter loop to process URLs until stop signal is received
	for {
		var idleChan <-chan time.Time

		w.logFunc(LogInfo, "waiting for pop...")

		// Initialize the idle timeout channel, if required
		if w.opts.WorkerIdleTTL > 0 {
			idleChan = time.After(w.opts.WorkerIdleTTL)
		}

		select {
		case <-w.stop:
			w.logFunc(LogInfo, "stop signal received.")
			return

		case <-idleChan:
			w.logFunc(LogInfo, "idle timeout received.")
			w.sendResponse(nil, false, nil, true)
			return

		case batch := <-w.pop:

			// Got a batch of urls to crawl, loop and check at each iteration if a stop
			// is received.
			for _, ctx := range batch {
				w.logFunc(LogInfo, "popped: %s", ctx.url)

				if ctx.IsRobotsURL() {
					w.requestRobotsTxt(ctx)
				} else if w.isAllowedPerRobotsPolicies(ctx.url) {
					w.requestURL(ctx, ctx.HeadBeforeGet)
				} else {
					// Must still notify Crawler that this URL was processed, although not visited
					w.opts.Extender.Disallowed(ctx)
					w.sendResponse(ctx, false, nil, false)
				}

				// No need to check for idle timeout here, no idling while looping through
				// a batch of URLs.
				select {
				case <-w.stop:
					w.logFunc(LogInfo, "stop signal received.")
					return
				default:
					// Nothing, just continue...
				}
			}
		}
	}
}

// Checks if the given URL can be fetched based on robots.txt policies.
func (w *worker) isAllowedPerRobotsPolicies(u *url.URL) bool {
	if w.robotsGroup != nil {
		// Is this URL allowed per robots.txt policy?
		ok := w.robotsGroup.Test(u.Path)
		if !ok {
			w.logFunc(LogIgnored, "ignored on robots.txt policy: %s", u.String())
		}
		return ok
	}

	// No robots.txt = everything is allowed
	return true
}

// Process the specified URL.
func (w *worker) requestURL(ctx *URLContext, headRequest bool) {
	if res, ok := w.fetchURL(ctx, w.opts.UserAgent, headRequest); ok {
		var harvested interface{}
		var visited bool

		// Close the body on function end
		defer res.Body.Close()

		// Any 2xx status code is good to go
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			// Success, visit the URL
			harvested = w.visitURL(ctx, res)
			visited = true
		} else {
			// Error based on status code received
			w.opts.Extender.Error(newCrawlErrorMessage(ctx, res.Status, CekHttpStatusCode))
			w.logFunc(LogError, "ERROR status code for %s: %s", ctx.url, res.Status)
		}
		w.sendResponse(ctx, visited, harvested, false)
	}
}

// Process the robots.txt URL.
func (w *worker) requestRobotsTxt(ctx *URLContext) {
	// Ask if it should be fetched
	if robData, reqRob := w.opts.Extender.RequestRobots(ctx, w.opts.RobotUserAgent); !reqRob {
		w.logFunc(LogInfo, "using robots.txt from cache")
		w.robotsGroup = w.getRobotsTxtGroup(ctx, robData, nil)

	} else if res, ok := w.fetchURL(ctx, w.opts.UserAgent, false); ok {
		// Close the body on function end
		defer res.Body.Close()
		w.robotsGroup = w.getRobotsTxtGroup(ctx, nil, res)
	}
}

// Get the robots.txt group for this crawler.
func (w *worker) getRobotsTxtGroup(ctx *URLContext, b []byte, res *http.Response) (g *robotstxt.Group) {
	var data *robotstxt.RobotsData
	var e error

	if res != nil {
		var buf bytes.Buffer
		io.Copy(&buf, res.Body)
		res.Body = ioutil.NopCloser(bytes.NewReader(buf.Bytes()))
		data, e = robotstxt.FromResponse(res)
		// Rewind the res.Body (by re-creating it from the bytes)
		res.Body = ioutil.NopCloser(bytes.NewReader(buf.Bytes()))
		// Error or not, the robots.txt has been fetched, so notify
		w.opts.Extender.FetchedRobots(ctx, res)
	} else {
		data, e = robotstxt.FromBytes(b)
	}

	// If robots data cannot be parsed, will return nil, which will allow access by default.
	// Reasonable, since by default no robots.txt means full access, so invalid
	// robots.txt is similar behavior.
	if e != nil {
		w.opts.Extender.Error(newCrawlError(nil, e, CekParseRobots))
		w.logFunc(LogError, "ERROR parsing robots.txt for host %s: %s", w.host, e)
	} else {
		g = data.FindGroup(w.opts.RobotUserAgent)
	}
	return g
}

// Set the crawl delay between this request and the next.
func (w *worker) setCrawlDelay() {
	var robDelay time.Duration

	if w.robotsGroup != nil {
		robDelay = w.robotsGroup.CrawlDelay
	}
	w.lastCrawlDelay = w.opts.Extender.ComputeDelay(w.host,
		&DelayInfo{
			w.opts.CrawlDelay,
			robDelay,
			w.lastCrawlDelay,
		},
		w.lastFetch)
	w.logFunc(LogInfo, "using crawl-delay: %v", w.lastCrawlDelay)
}

// Request the specified URL and return the response.
func (w *worker) fetchURL(ctx *URLContext, agent string, headRequest bool) (res *http.Response, ok bool) {
	var e error
	var silent bool

	for {
		// Wait for crawl delay, if one is pending.
		w.logFunc(LogTrace, "waiting for crawl delay")
		if w.wait != nil {
			<-w.wait
			w.wait = nil
		}

		// Compute the next delay
		w.setCrawlDelay()

		// Compute the fetch duration
		now := time.Now()

		// Request the URL
		if res, e = w.opts.Extender.Fetch(ctx, agent, headRequest); e != nil {
			// Check if this is an ErrEnqueueRedirect, in which case we will enqueue
			// the redirect-to URL.
			if ue, y := e.(*url.Error); y {
				// We have a *url.Error, check if it was returned because of an ErrEnqueueRedirect
				if ue.Err == ErrEnqueueRedirect {
					// Do not notify this error outside of this if block, this is not a
					// "real" error. We either enqueue the new URL, or fail to parse it,
					// and then stop processing the current URL.
					silent = true
					// Parse the URL in the context of the original URL (so that relative URLs are ok).
					// Absolute URLs that point to another host are ok too.
					if ur, e := ctx.url.Parse(ue.URL); e != nil {
						// Notify error
						w.opts.Extender.Error(newCrawlError(nil, e, CekParseRedirectURL))
						w.logFunc(LogError, "ERROR parsing redirect URL %s: %s", ue.URL, e)
					} else {
						// Enqueue the redirect-to URL
						w.logFunc(LogTrace, "redirect to %s", ur)
						w.enqueue <- ur
					}
				}
			}

			// No fetch, so set to nil
			w.lastFetch = nil

			if !silent {
				// Notify error
				w.opts.Extender.Error(newCrawlError(ctx, e, CekFetch))
				w.logFunc(LogError, "ERROR fetching %s: %s", ctx.url, e)
			}

			// Return from this URL crawl
			w.sendResponse(ctx, false, nil, false)
			return nil, false

		}
		// Get the fetch duration
		fetchDuration := time.Now().Sub(now)
		// Crawl delay starts now.
		w.wait = time.After(w.lastCrawlDelay)

		// Keep trace of this last fetch info
		w.lastFetch = &FetchInfo{
			ctx,
			fetchDuration,
			res.StatusCode,
			headRequest,
		}

		if headRequest {
			// Close the HEAD request's body
			defer res.Body.Close()
			// Next up is GET request, maybe
			headRequest = false
			// Ask caller if we should proceed with a GET
			if !w.opts.Extender.RequestGet(ctx, res) {
				w.logFunc(LogIgnored, "ignored on HEAD filter policy: %s", ctx.url)
				w.sendResponse(ctx, false, nil, false)
				ok = false
				break
			}
		} else {
			ok = true
			break
		}
	}
	return
}

// Send a response to the crawler.
func (w *worker) sendResponse(ctx *URLContext, visited bool, harvested interface{}, idleDeath bool) {
	// Push harvested urls back to crawler, even if empty (uses the channel communication
	// to decrement reference count of pending URLs)
	if ctx == nil || !isRobotsURL(ctx.url) {
		// If a stop signal has been received, ignore the response, since the push
		// channel may be full and could block indefinitely.
		select {
		case <-w.stop:
			w.logFunc(LogInfo, "ignoring send response, will stop.")
			return
		default:
			// Nothing, just continue...
		}

		// No stop signal, send the response
		res := &workerResponse{
			ctx,
			visited,
			harvested,
			w.host,
			idleDeath,
		}
		w.push <- res
	}
}

// Process the response for a URL.
func (w *worker) visitURL(ctx *URLContext, res *http.Response) interface{} {
	var doc *goquery.Document
	var harvested interface{}
	var doLinks bool

	// Load a goquery document and call the visitor function
	if bd, e := ioutil.ReadAll(res.Body); e != nil {
		w.opts.Extender.Error(newCrawlError(ctx, e, CekReadBody))
		w.logFunc(LogError, "ERROR reading body %s: %s", ctx.url, e)
	} else {
		if node, e := html.Parse(bytes.NewBuffer(bd)); e != nil {
			w.opts.Extender.Error(newCrawlError(ctx, e, CekParseBody))
			w.logFunc(LogError, "ERROR parsing %s: %s", ctx.url, e)
		} else {
			doc = goquery.NewDocumentFromNode(node)
			doc.Url = res.Request.URL
		}
		// Re-assign the body so it can be consumed by the visitor function
		res.Body = ioutil.NopCloser(bytes.NewBuffer(bd))
	}

	// Visit the document (with nil goquery doc if failed to load)
	if harvested, doLinks = w.opts.Extender.Visit(ctx, res, doc); doLinks {
		// Links were not processed by the visitor, so process links
		if doc != nil {
			harvested = w.processLinks(doc)
		} else {
			w.opts.Extender.Error(newCrawlErrorMessage(ctx, "No goquery document to process links.", CekProcessLinks))
			w.logFunc(LogError, "ERROR processing links %s", ctx.url)
		}
	}
	// Notify that this URL has been visited
	w.opts.Extender.Visited(ctx, harvested)

	return harvested
}

func handleBaseTag(rootURL string, baseHref string, aHref string) string {
	root, _ := url.Parse(rootURL)
	resolvedBase, _ := root.Parse(baseHref)

	parsedURL, _ := url.Parse(aHref)
	// If a[href] starts with a /, it overrides the base[href]
	if parsedURL.Host == "" && !strings.HasPrefix(aHref, "/") {
		aHref = path.Join(resolvedBase.Path, aHref)
	}

	resolvedURL, _ := resolvedBase.Parse(aHref)
	return resolvedURL.String()
}

// Scrape the document's content to gather all links
func (w *worker) processLinks(doc *goquery.Document) (result []*url.URL) {
	baseURL, _ := doc.Find("base[href]").Attr("href")
	urls := doc.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("href")
		if baseURL != "" {
			val = handleBaseTag(doc.Url.String(), baseURL, val)
		}
		return val
	})
	for _, s := range urls {
		// If href starts with "#", then it points to this same exact URL, ignore (will fail to parse anyway)
		if len(s) > 0 && !strings.HasPrefix(s, "#") {
			if parsed, e := url.Parse(s); e == nil {
				parsed = doc.Url.ResolveReference(parsed)
				result = append(result, parsed)
			} else {
				w.logFunc(LogIgnored, "ignore on unparsable policy %s: %s", s, e.Error())
			}
		}
	}
	return
}
