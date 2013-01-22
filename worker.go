package gocrawl

import (
	"bytes"
	"exp/html"
	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt.go"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The worker is dedicated to fetching and visiting a given host, respecting
// this host's robots.txt crawling policies.
type worker struct {
	// Worker identification
	host  string
	index int

	// User-agent strings
	userAgent      string
	robotUserAgent string

	// Communication channels and sync
	push    chan<- *workerResponse
	pop     popChannel
	stop    chan bool
	wg      *sync.WaitGroup
	wait    <-chan time.Time
	enqueue chan<- *CrawlerCommand

	// Config
	crawlDelay  time.Duration
	idleTTL     time.Duration
	robotsGroup *robotstxt.Group

	// Callbacks
	extender Extender
	logFunc  func(LogFlags, string, ...interface{})

	// Implementation fields
	lastFetch      *FetchInfo
	lastCrawlDelay time.Duration
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
		if this.idleTTL > 0 {
			idleChan = time.After(this.idleTTL)
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
			for _, cmd := range batch {
				this.logFunc(LogInfo, "popped: %s", cmd.u.String())

				if isRobotsTxtUrl(cmd.u) {
					this.requestRobotsTxt(cmd.u)
				} else if this.isAllowedPerRobotsPolicies(cmd.u) {
					this.requestUrl(cmd.u, cmd.head)
				} else {
					// Must still notify Crawler that this URL was processed, although not visited
					this.extender.Disallowed(cmd.u)
					this.sendResponse(cmd.u, false, nil, false)
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
func (this *worker) requestUrl(u *url.URL, headRequest bool) {
	if res, ok := this.fetchUrl(u, this.userAgent, headRequest); ok {
		var harvested []*url.URL
		var visited bool

		// Close the body on function end
		defer res.Body.Close()

		// Any 2xx status code is good to go
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			// Success, visit the URL
			harvested = this.visitUrl(res)
			visited = true
		} else {
			// Error based on status code received
			this.extender.Error(newCrawlErrorMessage(res.Status, CekHttpStatusCode, u))
			this.logFunc(LogError, "ERROR status code for %s: %s", u.String(), res.Status)
		}
		this.sendResponse(u, visited, harvested, false)
	}
}

// Process the robots.txt URL.
func (this *worker) requestRobotsTxt(u *url.URL) {
	// Ask if it should be fetched
	if reqRob, robData := this.extender.RequestRobots(u, this.robotUserAgent); !reqRob {
		this.logFunc(LogInfo, "using robots.txt from cache")
		this.robotsGroup = this.getRobotsTxtGroup(robData, nil)

	} else {
		// Fetch the document, using the robot user agent,
		// so that the host admin can see what robots are doing requests.
		if res, ok := this.fetchUrl(u, this.robotUserAgent, false); ok {
			// Close the body on function end
			defer res.Body.Close()
			this.robotsGroup = this.getRobotsTxtGroup(nil, res)
		}
	}
}

// Get the robots.txt group for this crawler.
func (this *worker) getRobotsTxtGroup(b []byte, res *http.Response) (g *robotstxt.Group) {
	var data *robotstxt.RobotsData
	var e error

	if res != nil {
		// Get the bytes from the response body
		b, e = ioutil.ReadAll(res.Body)
		// Rewind the res.Body (by re-creating it from the bytes)
		res.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		// Error or not, the robots.txt has been fetched, so notify
		this.extender.FetchedRobots(res)
	}

	if e == nil {
		data, e = robotstxt.FromBytes(b)
	}

	// If robots data cannot be parsed, will return nil, which will allow access by default.
	// Reasonable, since by default no robots.txt means full access, so invalid
	// robots.txt is similar behavior.
	if e != nil {
		this.extender.Error(newCrawlError(e, CekParseRobots, nil))
		this.logFunc(LogError, "ERROR parsing robots.txt for host %s: %s", this.host, e.Error())
	} else {
		g = data.FindGroup(this.robotUserAgent)
	}
	return
}

// Set the crawl delay between this request and the next.
func (this *worker) setCrawlDelay() {
	var robDelay time.Duration

	if this.robotsGroup != nil {
		robDelay = this.robotsGroup.CrawlDelay
	}
	this.lastCrawlDelay = this.extender.ComputeDelay(this.host,
		&DelayInfo{this.crawlDelay,
			robDelay,
			this.lastCrawlDelay},
		this.lastFetch)
	this.logFunc(LogInfo, "using crawl-delay: %v", this.lastCrawlDelay)
}

// Request the specified URL and return the response.
func (this *worker) fetchUrl(u *url.URL, agent string, headRequest bool) (res *http.Response, ok bool) {
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
		if res, e = this.extender.Fetch(u, agent, headRequest); e != nil {
			// Check if this is an EnqueueRedirectError, in which case we will enqueue
			// the redirect-to URL.
			if ue, y := e.(*url.Error); y {
				// We have a *url.Error, check if it was returned because of an EnqueueRedirectError
				if _, y = ue.Err.(*EnqueueRedirectError); y {
					// Do not notify this error outside of this if block, this is not a
					// "real" error. We either enqueue the new URL, or fail to parse it,
					// and then stop processing the current URL.
					silent = true
					// Parse the URL in the context of the original URL (so that relative URLs are ok).
					// Absolute URLs that point to another host are ok too.
					if ur, e := u.Parse(ue.URL); e != nil {
						// Notify error
						this.extender.Error(newCrawlError(e, CekParseRedirectUrl, nil))
						this.logFunc(LogError, "ERROR parsing redirect URL %s: %s", ue.URL, e.Error())
					} else {
						// Enqueue the redirect-to URL
						this.logFunc(LogTrace, "redirect to %s", ur.String())
						this.enqueue <- &CrawlerCommand{ur, EoRedirect}
					}
				}
			}

			// No fetch, so set to nil
			this.lastFetch = nil

			if !silent {
				// Notify error
				this.extender.Error(newCrawlError(e, CekFetch, u))
				this.logFunc(LogError, "ERROR fetching %s: %s", u.String(), e.Error())
			}

			// Return from this URL crawl
			this.sendResponse(u, false, nil, false)
			return nil, false

		} else {
			// Get the fetch duration
			fetchDuration := now.Sub(time.Now())
			// Crawl delay starts now.
			this.wait = time.After(this.lastCrawlDelay)

			// Keep trace of this last fetch info
			this.lastFetch = &FetchInfo{fetchDuration, res.StatusCode, headRequest, isRobotsTxtUrl(u)}
		}

		if headRequest {
			// Close the HEAD request's body
			defer res.Body.Close()
			// Next up is GET request, maybe
			headRequest = false
			// Ask caller if we should proceed with a GET
			if !this.extender.RequestGet(res) {
				this.logFunc(LogIgnored, "ignored on HEAD filter policy: %s", u.String())
				this.sendResponse(u, false, nil, false)
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
func (this *worker) sendResponse(u *url.URL, visited bool, harvested []*url.URL, idleDeath bool) {
	// Push harvested urls back to crawler, even if empty (uses the channel communication
	// to decrement reference count of pending URLs)
	if !isRobotsTxtUrl(u) {
		// If a stop signal has been received, ignore the response, since the push
		// channel may be full and could block indefinitely.
		select {
		case <-this.stop:
			this.logFunc(LogInfo, "ignoring send response, will stop.")
			// Put the signal back in stop channel, so that it is caught in the run() loop.
			this.stop <- true
			return
		default:
			// Nothing, just continue...
		}

		// No stop signal, send the response
		res := &workerResponse{this.host, visited, u, harvested, idleDeath}
		this.push <- res
	}
}

// Process the response for a URL.
func (this *worker) visitUrl(res *http.Response) []*url.URL {
	var doc *goquery.Document
	var harvested []*url.URL
	var doLinks bool

	// Load a goquery document and call the visitor function
	if bd, e := ioutil.ReadAll(res.Body); e != nil {
		this.extender.Error(newCrawlError(e, CekReadBody, res.Request.URL))
		this.logFunc(LogError, "ERROR reading body %s: %s", res.Request.URL.String(), e.Error())
	} else {
		if node, e := html.Parse(bytes.NewBuffer(bd)); e != nil {
			this.extender.Error(newCrawlError(e, CekParseBody, res.Request.URL))
			this.logFunc(LogError, "ERROR parsing %s: %s", res.Request.URL.String(), e.Error())
		} else {
			doc = goquery.NewDocumentFromNode(node)
			doc.Url = res.Request.URL
		}
		// Re-assign the body so it can be consumed by the visitor function
		res.Body = ioutil.NopCloser(bytes.NewBuffer(bd))
	}

	// Visit the document (with nil goquery doc if failed to load)
	if harvested, doLinks = this.extender.Visit(res, doc); doLinks {
		// Links were not processed by the visitor, so process links
		if doc != nil {
			harvested = this.processLinks(doc)
		} else {
			this.extender.Error(newCrawlErrorMessage("No goquery document to process links.", CekProcessLinks, res.Request.URL))
			this.logFunc(LogError, "ERROR processing links %s", res.Request.URL.String())
		}
	}
	// Notify that this URL has been visited
	this.extender.Visited(res.Request.URL, harvested)

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
