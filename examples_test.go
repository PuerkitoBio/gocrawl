package gocrawl

// import (
// 	"github.com/PuerkitoBio/goquery"
// 	"log"
// 	"net/http"
// 	"net/url"
// 	"os"
// 	"regexp"
// 	"time"
// )

// // Only enqueue the root and paths beginning with an "a"
// var rxOk = regexp.MustCompile(`http://duckduckgo\.com(/a.*)?$`)

// func visitor(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
// 	// Use the goquery document or res.Body to manipulate the data
// 	// ...

// 	// Return nil and true - let gocrawl find the links
// 	return nil, true
// }

// func urlSelector(u *url.URL, src *url.URL, isVisited bool) bool {
// 	return rxOk.MatchString(u.String())
// }

// func ExampleCrawl() {
// 	// Set custom options
// 	opts := NewOptions(visitor, urlSelector)
// 	opts.CrawlDelay = 1 * time.Second
// 	opts.LogFlags = LogInfo
// 	opts.Logger = log.New(os.Stdout, "", 0)

// 	// Play nice with ddgo when running the test!
// 	opts.MaxVisits = 2

// 	// Create crawler and start at root of duckduckgo
// 	c := NewCrawlerWithOptions(opts)
// 	c.Run("http://duckduckgo.com/")

// 	// Output: robot user-agent: Googlebot (gocrawl v0.1)
// 	// worker 1 launched for host duckduckgo.com
// 	// worker 1 - waiting for pop...
// 	// worker 1 - popped: http://duckduckgo.com/robots.txt
// 	// worker 1 - popped: http://duckduckgo.com
// 	// worker 1 - waiting for pop...
// 	// worker 1 - popped: http://duckduckgo.com/about.html
// 	// sending STOP signals...
// 	// waiting for goroutines to complete...
// 	// worker 1 - stop signal received.
// 	// worker 1 - worker done.
// 	// crawler done.
// }
