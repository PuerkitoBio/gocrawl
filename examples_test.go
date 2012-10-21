package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

// Only enqueue the root and paths beginning with an "a"
var rxOk = regexp.MustCompile(`http://duckduckgo\.com(/a.*)?$`)

// Create the Extender implementation, based on the gocrawl-provided DefaultExtender,
// because we don't want/need to override all methods.
type ExampleExtender struct {
	DefaultExtender // Will use the default implementation of all but Visit() and Filter()
}

// Override Visit for our need.
func (this *ExampleExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
	// Use the goquery document or res.Body to manipulate the data
	// ...

	// Return nil and true - let gocrawl find the links
	return nil, true
}

// Override Filter for our need.
func (this *ExampleExtender) Filter(u *url.URL, src *url.URL, isVisited bool) (bool, int) {
	// Priority (2nd return value) is ignored at the moment
	return rxOk.MatchString(u.String()), 0
}

func ExampleCrawl() {
	// Set custom options
	opts := NewOptions(new(ExampleExtender))
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogInfo
	opts.Logger = log.New(os.Stdout, "", 0)

	// Play nice with ddgo when running the test!
	opts.MaxVisits = 2

	// Create crawler and start at root of duckduckgo
	c := NewCrawlerWithOptions(opts)
	c.Run("http://duckduckgo.com/")

	// Output: robot user-agent: Googlebot (gocrawl v0.1)
	// worker 1 launched for host duckduckgo.com
	// worker 1 - waiting for pop...
	// worker 1 - popped: http://duckduckgo.com/robots.txt
	// worker 1 - using crawl-delay: 1s
	// worker 1 - popped: http://duckduckgo.com
	// worker 1 - using crawl-delay: 1s
	// worker 1 - waiting for pop...
	// worker 1 - popped: http://duckduckgo.com/about.html
	// worker 1 - using crawl-delay: 1s
	// sending STOP signals...
	// waiting for goroutines to complete...
	// worker 1 - stop signal received.
	// worker 1 - worker done.
	// crawler done.
}
