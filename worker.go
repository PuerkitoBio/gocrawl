package gocrawl

import (
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

type worker struct {
	visitor        func(*http.Response, *goquery.Document) ([]*url.URL, bool)
	push           chan<- *urlContainer
	pop            popChannel
	stop           chan bool
	userAgent      string
	robotUserAgent string
	logFunc        func(LogFlags, string, ...interface{})
	index          int
	wg             *sync.WaitGroup
	crawlDelay     time.Duration
	robotsData     *robotstxt.RobotsData
	fetcher        Fetcher
}

func (this *worker) Run() {
	defer func() {
		this.logFunc(LogTrace, "Done.\n")
		this.wg.Done()
	}()

	// Initialize the Fetcher to default if nil
	if this.fetcher == nil {
		this.fetcher = new(defaultFetcher)
	}

	// Enter loop to process URLs until stop signal is received
	for {
		this.logFunc(LogTrace, "Waiting for pop...\n")

		select {
		case <-this.stop:
			this.logFunc(LogTrace, "Stop signal received.\n")
			return

		case batch := <-this.pop:

			// Got a batch of urls to crawl, loop and check at each iteration if a stop 
			// is received.
			for _, u := range batch {
				this.logFunc(LogTrace, "Popped %s.\n", u.String())
				if this.isAllowedPerRobotsPolicies(u) {
					this.requestUrl(u)
				} else {
					// Must still notify Crawler that this URL was processed, although not visited
					this.notifyURLProcessed(u, nil)
				}

				select {
				case <-this.stop:
					this.logFunc(LogTrace, "Stop signal received.\n")
					return
				default:
					// Nothing, just continue...
				}
			}
		}
	}
}

func (this *worker) isAllowedPerRobotsPolicies(u *url.URL) bool {
	if this.robotsData != nil {
		// Is this URL allowed per robots.txt policy?
		ok, e := this.robotsData.TestAgent(u.Path, this.robotUserAgent)
		if e != nil {
			this.logFunc(LogTrace, "RobotsData returned error %s, deny access to url %s\n", e.Error(), u.String())
			ok = false
		} else if !ok {
			this.logFunc(LogTrace, "Access denied per RobotsData policy to url %s\n", u.String())
		} else {
			this.logFunc(LogTrace, "Access allowed per RobotsData policy to url %s\n", u.String())
		}
		return ok

	} else if !isRobotsTxtUrl(u) {
		this.logFunc(LogError, "Error no Robots.txt data for url %s\n", u.String())
		// TODO : Go ahead and request anyway?
	}

	// Defaults to true (?)
	return true
}

func (this *worker) requestUrl(u *url.URL) {
	// Fetch the document
	if res, e := this.fetcher.Fetch(u, this.userAgent); e != nil {
		this.logFunc(LogError, "Error GET for url %s: %s\n", u.String(), e.Error())
		this.notifyURLProcessed(u, nil)

	} else {
		var harvested []*url.URL

		// Close the body on function end
		defer res.Body.Close()

		// Crawl delay starts now
		wait := time.After(this.crawlDelay)

		// Special case if this is the robots.txt
		if isRobotsTxtUrl(u) {
			if body, e := ioutil.ReadAll(res.Body); e != nil {
				this.logFunc(LogError, "Error reading robots.txt body for host %s: %s\n", u.Host, e.Error())
				// TODO : Can't really continue, panic?

			} else if data, e := robotstxt.FromResponseBytes(res.StatusCode, body, false); e != nil {
				this.logFunc(LogError, "Error parsing robots.txt for host %s: %s\n", u.Host, e.Error())
				// TODO : Can't really continue, panic?
			} else {
				this.logFunc(LogTrace, "Caching robots.txt data for host %s\n", u.Host)
				this.robotsData = data
			}
		} else {
			// Normal path
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				// Success, visit the URL
				harvested = this.visitUrl(res)
			} else {
				// Error based on status code received
				this.logFunc(LogError, "Error returned from server for url %s: %s\n", u.String(), res.Status)
			}
		}
		this.notifyURLProcessed(u, harvested)

		// Wait for crawl delay
		<-wait
	}
}

func (this *worker) notifyURLProcessed(u *url.URL, harvested []*url.URL) {
	// Do NOT notify for robots.txt URLs, this is an under-the-cover request,
	// not an actual URL enqueued for crawling.
	if !isRobotsTxtUrl(u) {
		// Push harvested urls back to crawler, even if empty (uses the channel communication
		// to decrement reference count of pending URLs)
		this.push <- &urlContainer{u, harvested}
	}
}

func (this *worker) visitUrl(res *http.Response) []*url.URL {
	var doc *goquery.Document
	var harvested []*url.URL
	var doLinks bool

	// Load a goquery document and call the visitor function
	if node, e := html.Parse(res.Body); e != nil {
		this.logFunc(LogError, "Error parsing HTML for url %s: %s\n", res.Request.URL.String(), e.Error())
	} else {
		doc = goquery.NewDocumentFromNode(node)
		doc.Url = res.Request.URL
	}

	// Visit the document (with nil goquery doc if failed to load)
	if harvested, doLinks = this.visitor(res, doc); doLinks && doc != nil {
		// Links were not processed by the visitor, so process links
		harvested = this.processLinks(doc)
	}

	return harvested
}

func (this *worker) processLinks(doc *goquery.Document) (result []*url.URL) {
	urls := doc.Root.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
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
				this.logFunc(LogTrace, "URL ignored, unparsable %s: %s\n", s, e.Error())
			}
		}
	}
	return
}
