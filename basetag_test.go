package gocrawl

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

var visitedPage2 bool = false
var visitedPage3 bool = false

type BaseTagExtender struct {
	DefaultExtender // Will use the default implementation of all but Visit() and Filter()
}

func (this *BaseTagExtender) Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
	// fmt.Println("ctx:", ctx.NormalizedURL().String(), "Ends with:", strings.HasSuffix(ctx.NormalizedURL().String(), "page2.html"))
	if strings.HasSuffix(ctx.NormalizedURL().String(), "page2.html") {
		visitedPage2 = true
	}
	if strings.HasSuffix(ctx.NormalizedURL().String(), "page3.html") {
		visitedPage3 = true
	}
	// Return nil and true - let gocrawl find the links
	return nil, true
}

func TestBaseTag(t *testing.T) {
	assertCnt := 0
	assertTrue = func(cond bool, msg string, args ...interface{}) bool {
		assertCnt++
		if !cond {
			t.Errorf("FAIL %s - %s.", "TestBaseTag", fmt.Sprintf(msg, args...))
			return false
		}
		return true
	}
	opts := NewOptions(new(BaseTagExtender))
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogAll

	// Play nice with ddgo when running the test!
	opts.MaxVisits = 2
	http.Handle("/", http.FileServer(http.Dir("./testdata/hostd")))

	// Create server
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Fatal(err)
	}

	// Start server in a separate goroutine
	go func() {
		http.Serve(l, nil)
	}()

	c := NewCrawlerWithOptions(opts)
	c.Run("http://localhost:8080/subdir/page1.html")

	assertTrue(visitedPage2, "expected visitedPage2")
	assertTrue(visitedPage3, "expected visitedPage3")

	// Close listener
	if err = l.Close(); err != nil {
		panic(err)
	}

}
