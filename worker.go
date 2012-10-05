package gocrawl

import (
	"exp/html"
	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt.go"
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
	robotsGroup    *robotstxt.Group
	fetcher        Fetcher
}

func (this *worker) run() {
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
					this.notifyURLProcessed(u, false, nil)
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
	if this.robotsGroup != nil {
		// Is this URL allowed per robots.txt policy?
		ok := this.robotsGroup.Test(u.Path)
		if !ok {
			this.logFunc(LogTrace, "Access denied per RobotsData policy to url %s\n", u.String())
		} else {
			this.logFunc(LogTrace, "Access allowed per RobotsData policy to url %s\n", u.String())
		}
		return ok

	} else {
		// No robots.txt group for this user-agent means allow access by default
		this.logFunc(LogTrace, "No Robots.txt data for user-agent %s\n", this.robotUserAgent)
	}

	return true
}

func (this *worker) requestUrl(u *url.URL) {
	// Fetch the document
	if res, e := this.fetcher.Fetch(u, this.userAgent); e != nil {
		this.logFunc(LogError, "Error GET for url %s: %s\n", u.String(), e.Error())
		this.notifyURLProcessed(u, false, nil)

	} else {
		var harvested []*url.URL
		var visited bool

		// Close the body on function end
		defer res.Body.Close()

		// Crawl delay starts now
		wait := time.After(this.crawlDelay)

		// Special case if this is the robots.txt
		if isRobotsTxtUrl(u) {
			if data, e := robotstxt.FromResponse(res); e != nil {
				// this.robotsGroup will be nil, which will allow access by default.
				// Reasonable, since by default no robots.txt means full access, so invalid
				// robots.txt is similar behavior.
				this.logFunc(LogError, "Error parsing robots.txt for host %s: %s\n", u.Host, e.Error())
			} else {
				this.logFunc(LogTrace, "Caching robots.txt group for host %s\n", u.Host)
				if this.robotsGroup = data.FindGroup(this.robotUserAgent); this.robotsGroup != nil {
					// Use robots.txt crawl-delay, if specified
					if this.robotsGroup.CrawlDelay > 0 {
						this.crawlDelay = this.robotsGroup.CrawlDelay
						this.logFunc(LogTrace, "Setting crawl-delay to %v\n", this.crawlDelay)
					}
				}
			}
		} else {
			// Normal path
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				// Success, visit the URL
				harvested = this.visitUrl(res)
				visited = true
			} else {
				// Error based on status code received
				this.logFunc(LogError, "Error returned from server for url %s: %s\n", u.String(), res.Status)
			}
		}
		this.notifyURLProcessed(u, visited, harvested)

		// Wait for crawl delay
		<-wait
	}
}

func (this *worker) notifyURLProcessed(u *url.URL, visited bool, harvested []*url.URL) {
	// Do NOT notify for robots.txt URLs, this is an under-the-cover request,
	// not an actual URL enqueued for crawling.
	if !isRobotsTxtUrl(u) {
		// Push harvested urls back to crawler, even if empty (uses the channel communication
		// to decrement reference count of pending URLs)
		this.push <- &urlContainer{u, visited, harvested}
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
	if this.visitor != nil {
		if harvested, doLinks = this.visitor(res, doc); doLinks && doc != nil {
			// Links were not processed by the visitor, so process links
			harvested = this.processLinks(doc)
		}
	} else {
		this.logFunc(LogTrace, "No visitor function, url not visited %s\n", res.Request.URL.String())
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
