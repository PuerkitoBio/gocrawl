package gocrawl

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt.go"
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
func (this *worker) run() {
	defer func() {
		this.logFunc(LogInfo, "worker done.")
		this.wg.Done()
	}()

	// Enter loop to process URLs until stop signal is received
	for {
		var idleChan <-chan time.Time

		this.logFunc(LogInfo, "waiting for pop...")

		// Initialize the idle timeout channel, if required
		if this.opts.WorkerIdleTTL > 0 {
			idleChan = time.After(this.opts.WorkerIdleTTL)
		}

		select {
		case <-this.stop:
			this.logFunc(LogInfo, "stop signal received.")
			return

		case <-idleChan:
			this.logFunc(LogInfo, "idle timeout received.")
			this.sendResponse(nil, false, nil, true)
			return

		case batch := <-this.pop:

			// Got a batch of urls to crawl, loop and check at each iteration if a stop
			// is received.
			for _, ctx := range batch {
				this.logFunc(LogInfo, "popped: %s", ctx.url)

				if ctx.IsRobotsURL() {
					this.requestRobotsTxt(ctx)
				} else if this.isAllowedPerRobotsPolicies(ctx.url) {
					this.requestUrl(ctx, ctx.HeadBeforeGet)
				} else {
					// Must still notify Crawler that this URL was processed, although not visited
					this.opts.Extender.Disallowed(ctx)
					this.sendResponse(ctx, false, nil, false)
				}

				// No need to check for idle timeout here, no idling while looping through
				// a batch of URLs.
				select {
				case <-this.stop:
					this.logFunc(LogInfo, "stop signal received.")
					return
				default:
					// Nothing, just continue...
				}
			}
		}
	}
}

// Checks if the given URL can be fetched based on robots.txt policies.
func (this *worker) isAllowedPerRobotsPolicies(u *url.URL) bool {
	if this.robotsGroup != nil {
		// Is this URL allowed per robots.txt policy?
		ok := this.robotsGroup.Test(u.Path)
		if !ok {
			this.logFunc(LogIgnored, "ignored on robots.txt policy: %s", u.String())
		}
		return ok
	}

	// No robots.txt = everything is allowed
	return true
}

// Process the specified URL.
func (this *worker) requestUrl(ctx *URLContext, headRequest bool) {
	if res, ok := this.fetchUrl(ctx, this.opts.UserAgent, headRequest); ok {
		var harvested interface{}
		var visited bool

		// Close the body on function end
		defer res.Body.Close()

		// Any 2xx status code is good to go
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			// Success, visit the URL
			harvested = this.visitUrl(ctx, res)
			visited = true
		} else {
			// Error based on status code received
			this.opts.Extender.Error(newCrawlErrorMessage(ctx, res.Status, CekHttpStatusCode))
			this.logFunc(LogError, "ERROR status code for %s: %s", ctx.url, res.Status)
		}
		this.sendResponse(ctx, visited, harvested, false)
	}
}

// Process the robots.txt URL.
func (this *worker) requestRobotsTxt(ctx *URLContext) {
	// Ask if it should be fetched
	if robData, reqRob := this.opts.Extender.RequestRobots(ctx, this.opts.RobotUserAgent); !reqRob {
		this.logFunc(LogInfo, "using robots.txt from cache")
		this.robotsGroup = this.getRobotsTxtGroup(ctx, robData, nil)

	} else {
		// Fetch the document, using the robot user agent,
		// so that the host admin can see what robots are doing requests.
		if res, ok := this.fetchUrl(ctx, this.opts.RobotUserAgent, false); ok {
			// Close the body on function end
			defer res.Body.Close()
			this.robotsGroup = this.getRobotsTxtGroup(ctx, nil, res)
		}
	}
}

// Get the robots.txt group for this crawler.
func (this *worker) getRobotsTxtGroup(ctx *URLContext, b []byte, res *http.Response) (g *robotstxt.Group) {
	var data *robotstxt.RobotsData
	var e error

	if res != nil {
		// Get the bytes from the response body
		b, e = ioutil.ReadAll(res.Body)
		// Rewind the res.Body (by re-creating it from the bytes)
		res.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		// Error or not, the robots.txt has been fetched, so notify
		this.opts.Extender.FetchedRobots(ctx, res)
	}

	if e == nil {
		data, e = robotstxt.FromBytes(b)
	}

	// If robots data cannot be parsed, will return nil, which will allow access by default.
	// Reasonable, since by default no robots.txt means full access, so invalid
	// robots.txt is similar behavior.
	if e != nil {
		this.opts.Extender.Error(newCrawlError(nil, e, CekParseRobots))
		this.logFunc(LogError, "ERROR parsing robots.txt for host %s: %s", this.host, e)
	} else {
		g = data.FindGroup(this.opts.RobotUserAgent)
	}
	return
}

// Set the crawl delay between this request and the next.
func (this *worker) setCrawlDelay() {
	var robDelay time.Duration

	if this.robotsGroup != nil {
		robDelay = this.robotsGroup.CrawlDelay
	}
	this.lastCrawlDelay = this.opts.Extender.ComputeDelay(this.host,
		&DelayInfo{
			this.opts.CrawlDelay,
			robDelay,
			this.lastCrawlDelay,
		},
		this.lastFetch)
	this.logFunc(LogInfo, "using crawl-delay: %v", this.lastCrawlDelay)
}

// Request the specified URL and return the response.
func (this *worker) fetchUrl(ctx *URLContext, agent string, headRequest bool) (res *http.Response, ok bool) {
	var e error
	var silent bool

	for {
		// Wait for crawl delay, if one is pending.
		this.logFunc(LogTrace, "waiting for crawl delay")
		if this.wait != nil {
			<-this.wait
			this.wait = nil
		}

		// Compute the next delay
		this.setCrawlDelay()

		// Compute the fetch duration
		now := time.Now()

		// Request the URL
		if res, e = this.opts.Extender.Fetch(ctx, agent, headRequest); e != nil {
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
						this.opts.Extender.Error(newCrawlError(nil, e, CekParseRedirectURL))
						this.logFunc(LogError, "ERROR parsing redirect URL %s: %s", ue.URL, e)
					} else {
						// Enqueue the redirect-to URL
						this.logFunc(LogTrace, "redirect to %s", ur)
						this.enqueue <- ur
					}
				}
			}

			// No fetch, so set to nil
			this.lastFetch = nil

			if !silent {
				// Notify error
				this.opts.Extender.Error(newCrawlError(ctx, e, CekFetch))
				this.logFunc(LogError, "ERROR fetching %s: %s", ctx.url, e)
			}

			// Return from this URL crawl
			this.sendResponse(ctx, false, nil, false)
			return nil, false

		} else {
			// Get the fetch duration
			fetchDuration := now.Sub(time.Now())
			// Crawl delay starts now.
			this.wait = time.After(this.lastCrawlDelay)

			// Keep trace of this last fetch info
			this.lastFetch = &FetchInfo{
				ctx,
				fetchDuration,
				res.StatusCode,
				headRequest,
			}
		}

		if headRequest {
			// Close the HEAD request's body
			defer res.Body.Close()
			// Next up is GET request, maybe
			headRequest = false
			// Ask caller if we should proceed with a GET
			if !this.opts.Extender.RequestGet(ctx, res) {
				this.logFunc(LogIgnored, "ignored on HEAD filter policy: %s", ctx.url)
				this.sendResponse(ctx, false, nil, false)
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
func (this *worker) sendResponse(ctx *URLContext, visited bool, harvested interface{}, idleDeath bool) {
	// Push harvested urls back to crawler, even if empty (uses the channel communication
	// to decrement reference count of pending URLs)
	if ctx == nil || !isRobotsURL(ctx.url) {
		// If a stop signal has been received, ignore the response, since the push
		// channel may be full and could block indefinitely.
		select {
		case <-this.stop:
			this.logFunc(LogInfo, "ignoring send response, will stop.")
			return
		default:
			// Nothing, just continue...
		}

		// No stop signal, send the response
		res := &workerResponse{
			ctx,
			visited,
			harvested,
			this.host,
			idleDeath,
		}
		this.push <- res
	}
}

// Process the response for a URL.
func (this *worker) visitUrl(ctx *URLContext, res *http.Response) interface{} {
	var doc *goquery.Document
	var harvested interface{}
	var doLinks bool

	// Load a goquery document and call the visitor function
	if bd, e := ioutil.ReadAll(res.Body); e != nil {
		this.opts.Extender.Error(newCrawlError(ctx, e, CekReadBody))
		this.logFunc(LogError, "ERROR reading body %s: %s", ctx.url, e)
	} else {
		if node, e := html.Parse(bytes.NewBuffer(bd)); e != nil {
			this.opts.Extender.Error(newCrawlError(ctx, e, CekParseBody))
			this.logFunc(LogError, "ERROR parsing %s: %s", ctx.url, e)
		} else {
			doc = goquery.NewDocumentFromNode(node)
			doc.Url = res.Request.URL
		}
		// Re-assign the body so it can be consumed by the visitor function
		res.Body = ioutil.NopCloser(bytes.NewBuffer(bd))
	}

	// Visit the document (with nil goquery doc if failed to load)
	if harvested, doLinks = this.opts.Extender.Visit(ctx, res, doc); doLinks {
		// Links were not processed by the visitor, so process links
		if doc != nil {
			harvested = this.processLinks(doc)
		} else {
			this.opts.Extender.Error(newCrawlErrorMessage(ctx, "No goquery document to process links.", CekProcessLinks))
			this.logFunc(LogError, "ERROR processing links %s", ctx.url)
		}
	}
	// Notify that this URL has been visited
	this.opts.Extender.Visited(ctx, harvested)

	return harvested
}

// Scrape the document's content to gather all links
func (this *worker) processLinks(doc *goquery.Document) (result []*url.URL) {
	urls := doc.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("href")
		return val
	})
	for _, s := range urls {
		// If href starts with "#", then it points to this same exact URL, ignore (will fail to parse anyway)
		if len(s) > 0 && !strings.HasPrefix(s, "#") {
			if parsed, e := url.Parse(s); e == nil {
				parsed = doc.Url.ResolveReference(parsed)
				result = append(result, parsed)
			} else {
				this.logFunc(LogIgnored, "ignore on unparsable policy %s: %s", s, e.Error())
			}
		}
	}
	return
}
