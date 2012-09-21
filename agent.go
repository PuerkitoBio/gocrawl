package gocrawl

import (
	"exp/html"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"net/url"
)

var httpClient http.Client

type agent struct {
	visitor   func(*http.Response, *goquery.Document) ([]*url.URL, bool)
	push      chan *urlContainer
	pop       popChannel
	userAgent string
	logger    *log.Logger
	logLevel  LogLevel
	index     int
}

func (this *agent) Run() {
	defer func() {
		if this.logLevel|LogTrace == LogTrace {
			this.logger.Printf("Agent %d - Done.\n", this.index)
		}
	}()

	// Run until channel is closed
	if this.logLevel|LogTrace == LogTrace {
		this.logger.Printf("Agent %d - Waiting for pop...\n", this.index)
	}
	for u, ok := this.pop.get(); ok; u, ok = this.pop.get() {
		if this.logLevel|LogTrace == LogTrace {
			this.logger.Printf("Agent %d - Popped %s.\n", this.index, u.String())
		}
		this.requestUrl(u)
		if this.logLevel|LogTrace == LogTrace {
			this.logger.Printf("Agent %d - Waiting for pop...\n", this.index)
		}
	}
}

func (this *agent) requestUrl(u *url.URL) {
	// Prepare the request with the right user agent
	if req, e := http.NewRequest("GET", u.String(), nil); e != nil {
		if this.logLevel|LogError == LogError {
			this.logger.Printf("Agent %d - Error creating request for url %s: %s\n", this.index, u.String(), e.Error())
		}
	} else {
		req.Header["User-Agent"] = []string{this.userAgent}

		// Send the request
		if res, e := httpClient.Do(req); e != nil {
			if this.logLevel|LogError == LogError {
				this.logger.Printf("Agent %d - Error GET for url %s: %s\n", this.index, u.String(), e.Error())
			}
		} else {
			defer res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				// Success, visit the URL
				this.visitUrl(res)
			} else {
				// Error based on status code received
				if this.logLevel|LogError == LogError {
					this.logger.Printf("Agent %d - Error returned from server for url %s: %s\n", this.index, u.String(), res.Status)
				}
			}
		}
	}
}

func (this *agent) visitUrl(res *http.Response) {
	var doc *goquery.Document
	var harvested []*url.URL
	var doLinks bool

	// Load a goquery document and call the visitor function
	if node, e := html.Parse(res.Body); e != nil {
		if this.logLevel|LogError == LogError {
			this.logger.Printf("Agent %d - Error parsing HTML for url %s: %s\n", this.index, res.Request.URL.String(), e.Error())
		}
	} else {
		doc = goquery.NewDocumentFromNode(node)
		doc.Url = res.Request.URL
	}

	// Visit the document
	if harvested, doLinks = this.visitor(res, doc); doLinks && doc != nil {
		// Links were not processed by the visitor, so process links
		harvested = this.processLinks(doc)
	}

	// Push harvested urls back to crawler, even if empty (uses the channel communication
	// to decrement reference count of pending URLs)
	this.push <- &urlContainer{res.Request.URL, harvested}
}

func (this *agent) processLinks(doc *goquery.Document) (result []*url.URL) {
	urls := doc.Root.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("href")
		return val
	})
	for _, s := range urls {
		if parsed, e := url.Parse(s); e == nil {
			parsed = doc.Url.ResolveReference(parsed)
			result = append(result, parsed)
		} else {
			if this.logLevel|LogTrace == LogTrace {
				this.logger.Printf("Agent %d - URL ignored, unparsable %s: %s\n", this.index, s, e.Error())
			}
		}
	}
	return
}
