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
	visitor VisitorFunc
	push    chan *urlContainer
	pop     popChannel
	logger  *log.Logger
	index   int
}

func (this *agent) Run() {
	var res *http.Response
	var node *html.Node
	var e error

	// TODO : Refactor in smaller funcs, defer res.Body.Close
	defer this.logger.Printf("Agent %d - Done.\n", this.index)

	// Run until channel is closed
	this.logger.Printf("Agent %d - Waiting for pop...\n", this.index)

	for u, ok := this.pop.get(); ok; u, ok = this.pop.get() {

		this.logger.Printf("Agent %d - Popped %s.\n", this.index, u.String())

		if res, e = httpClient.Get(u.String()); e != nil {
			this.logger.Printf("Agent %d - Error GET for url %s: %s\n", this.index, u.String(), e.Error())
		} else {
			//this.logger.Printf("Agent %d - GET %s: status %d\n", this.index, url.String(), res.StatusCode)
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				var doc *goquery.Document
				var harvested []*url.URL
				var do bool

				// Load a goquery document and call the visitor function
				if node, e = html.Parse(res.Body); e != nil {
					this.logger.Printf("Agent %d - Error parsing HTML for url %s: %s\n", this.index, u.String(), e.Error())
				} else {
					doc = goquery.NewDocumentFromNode(node)
				}

				// Visit the document
				if harvested, do = this.visitor(res, doc); do && doc != nil {
					// Links were not processed by the visitor, so process links
					harvested = func() (result []*url.URL) {
						urls := doc.Root.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
							val, _ := s.Attr("href")
							//this.logger.Printf("Agent %d - Link found in %s : %s\n", this.index, u.String(), val)
							return val
						})
						for _, s := range urls {
							if parsed, e := url.Parse(s); e == nil {
								parsed = u.ResolveReference(parsed)
								result = append(result, parsed)
							}
						}
						return
					}()
				}

				// Push harvested urls back to crawler, even if empty (uses the channel communication
				// to decrement reference count of pending URLs)
				this.push <- &urlContainer{u, harvested}
			} else {
				this.logger.Printf("Agent %d - Error returned from server for url %s: %s\n", this.index, u.String(), res.Status)
			}
		}
		this.logger.Printf("Agent %d - Waiting for pop...\n", this.index)
	}
}
