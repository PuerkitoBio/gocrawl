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
	push chan<- *workerResponse
	pop  popChannel
	stop chan bool
	wg   *sync.WaitGroup

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
		this.logFunc(LogInfo, "worker done.\n")
		this.wg.Done()
	}()

	// Enter loop to process URLs until stop signal is received
	for {
		var idleChan <-chan time.Time

		this.logFunc(LogInfo, "waiting for pop...\n")

		// Initialize the idle timeout channel, if required
		if this.idleTTL > 0 {
			idleChan = time.After(this.idleTTL)
		}

		select {
		case <-this.stop:
			this.logFunc(LogInfo, "stop signal received.\n")
			return

		case <-idleChan:
			this.logFunc(LogInfo, "idle timeout received.\n")
			this.sendResponse(nil, false, nil, true)
			return

		case batch := <-this.pop:

			// Got a batch of urls to crawl, loop and check at each iteration if a stop 
			// is received.
			for _, cmd := range batch {
				var robDelay time.Duration

				this.logFunc(LogInfo, "popped: %s\n", cmd.u.String())

				// Get the crawl delay between this request and the next
				if this.robotsGroup != nil {
					robDelay = this.robotsGroup.CrawlDelay
				}
				this.lastCrawlDelay = this.extender.ComputeDelay(this.host,
					&DelayInfo{this.crawlDelay,
						robDelay,
						this.lastCrawlDelay},
					this.lastFetch)
				this.logFunc(LogInfo, "using crawl-delay: %v\n", this.lastCrawlDelay)

				if isRobotsTxtUrl(cmd.u) {
					this.requestRobotsTxt(cmd.u, this.lastCrawlDelay)
				} else if this.isAllowedPerRobotsPolicies(cmd.u) {
					this.requestUrl(cmd.u, this.lastCrawlDelay)
				} else {
					// Must still notify Crawler that this URL was processed, although not visited
					this.extender.Disallowed(cmd.u)
					this.sendResponse(cmd.u, false, nil, false)
				}

				// No need to check for idle timeout here, no idling while looping through
				// a batch of URLs.
				select {
				case <-this.stop:
					this.logFunc(LogInfo, "stop signal received.\n")
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
			this.logFunc(LogIgnored, "ignored on robots.txt policy: %s\n", u.String())
		}
		return ok
	}

	// No robots.txt = everything is allowed
	return true
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
		this.logFunc(LogError, "ERROR parsing robots.txt for host %s: %s\n", this.host, e.Error())
	} else {
		g = data.FindGroup(this.robotUserAgent)
	}
	return
}

// Process the robots.txt URL.
func (this *worker) requestRobotsTxt(u *url.URL, delay time.Duration) {
	// Ask if it should be fetched
	if reqRob, robData := this.extender.RequestRobots(u, this.robotUserAgent); !reqRob {
		this.logFunc(LogInfo, "using robots.txt from cache\n")
		this.robotsGroup = this.getRobotsTxtGroup(robData, nil)

		// Fetch the document, using the robot user agent,
		// so that the host admin can see what robots are doing requests.
	} else if res, ok := this.fetchUrl(u, this.robotUserAgent); ok {
		// Close the body on function end
		defer res.Body.Close()

		// Crawl delay starts now
		wait := time.After(delay)

		this.robotsGroup = this.getRobotsTxtGroup(nil, res)

		// Wait for crawl delay
		<-wait
	}
}

// Request the specified URL and return the response.
func (this *worker) fetchUrl(u *url.URL, agent string) (res *http.Response, ok bool) {
	var e error

	// Compute the fetch duration
	now := time.Now()

	// Request the URL
	if res, e = this.extender.Fetch(u, agent, false); e != nil {
		// No fetch, so set to nil
		this.lastFetch = nil

		// Notify error
		this.extender.Error(newCrawlError(e, CekFetch, u))
		this.logFunc(LogError, "ERROR fetching %s: %s\n", u.String(), e.Error())

		// Return from this URL crawl
		this.sendResponse(u, false, nil, false)
	} else {
		this.lastFetch = &FetchInfo{now.Sub(time.Now()), res.StatusCode, false, isRobotsTxtUrl(u)}
		ok = true
	}
	return
}

// Process the specified URL.
func (this *worker) requestUrl(u *url.URL, delay time.Duration) {
	if res, ok := this.fetchUrl(u, this.userAgent); ok {
		var harvested []*url.URL
		var visited bool

		// Close the body on function end
		defer res.Body.Close()

		// Crawl delay starts now
		wait := time.After(delay)

		// Any 2xx status code is good to go
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			// Success, visit the URL
			harvested = this.visitUrl(res)
			visited = true
		} else {
			// Error based on status code received
			this.extender.Error(newCrawlErrorMessage(res.Status, CekHttpStatusCode, u))
			this.logFunc(LogError, "ERROR status code for %s: %s\n", u.String(), res.Status)
		}
		this.sendResponse(u, visited, harvested, false)

		// Wait for crawl delay
		<-wait
	}
}

// Send a response to the crawler.
func (this *worker) sendResponse(u *url.URL, visited bool, harvested []*url.URL, idleDeath bool) {
	// Push harvested urls back to crawler, even if empty (uses the channel communication
	// to decrement reference count of pending URLs)
	if !isRobotsTxtUrl(u) {
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
		this.logFunc(LogError, "ERROR reading body %s: %s\n", res.Request.URL.String(), e.Error())
	} else {
		if node, e := html.Parse(bytes.NewBuffer(bd)); e != nil {
			this.extender.Error(newCrawlError(e, CekParseBody, res.Request.URL))
			this.logFunc(LogError, "ERROR parsing %s: %s\n", res.Request.URL.String(), e.Error())
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
			this.logFunc(LogError, "ERROR processing links %s\n", res.Request.URL.String())
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
				this.logFunc(LogIgnored, "ignore on unparsable policy %s: %s\n", s, e.Error())
			}
		}
	}
	return
}
