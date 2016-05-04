package gocrawl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var visitedPage2 = false
var visitedPage3 = false
var visitedPageA = false
var visitedPageB = false

type BaseTagExtender struct {
	DefaultExtender // Will use the default implementation of all but Visit() and Filter()
}

func (x *BaseTagExtender) Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
	if strings.HasSuffix(ctx.NormalizedURL().String(), "page2.html") {
		visitedPage2 = true
	}
	if strings.HasSuffix(ctx.NormalizedURL().String(), "page3.html") {
		visitedPage3 = true
	}
	if strings.HasSuffix(ctx.NormalizedURL().String(), "pagea.html") {
		visitedPageA = true
	}
	if strings.HasSuffix(ctx.NormalizedURL().String(), "pageb.html") {
		visitedPageB = true
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

	srv := httptest.NewServer(http.FileServer(http.Dir("./testdata/hostd")))
	defer srv.Close()

	c := NewCrawlerWithOptions(opts)
	c.Run(srv.URL + "/subdir/page1.html")

	assertTrue(visitedPage2, "Expected page2.html to be visited")
	assertTrue(visitedPage3, "Expected page3.html to be visited")

	c.Run(srv.URL + "/subdir/pagea.html")

	assertTrue(visitedPageA, "Expected pagea.html to be visited")
	assertTrue(visitedPageB, "Expected pageb.html to be visited")
}

func TestHandleBaseTag(t *testing.T) {
	rootURL, _ := url.Parse("http://example.com")

	var testCases = [][]string{
		// base[href], a[href], expected destination

		[]string{"http://other.com", "http://host.com/a", "http://host.com/a"},
		[]string{"http://other.com", "//host.com", "http://host.com"},
		[]string{"http://other.com", "//host.com/", "http://host.com/"},
		[]string{"http://other.com", "//host.com/b", "http://host.com/b"},
		[]string{"http://other.com", "/", "http://other.com/"},
		[]string{"http://other.com", "/sub", "http://other.com/sub"},
		[]string{"http://other.com", "/sub/", "http://other.com/sub/"},

		[]string{"http://other.com/", "http://host.com/a", "http://host.com/a"},
		[]string{"http://other.com/", "//host.com", "http://host.com"},
		[]string{"http://other.com/", "//host.com/", "http://host.com/"},
		[]string{"http://other.com/", "//host.com/b", "http://host.com/b"},
		[]string{"http://other.com/", "/", "http://other.com/"},
		[]string{"http://other.com/", "/sub", "http://other.com/sub"},
		[]string{"http://other.com/", "/sub/", "http://other.com/sub/"},

		[]string{"//other.com", "http://host.com/a", "http://host.com/a"},
		[]string{"//other.com", "//host.com", "http://host.com"},
		[]string{"//other.com", "//host.com/", "http://host.com/"},
		[]string{"//other.com", "//host.com/b", "http://host.com/b"},
		[]string{"//other.com", "/", "http://other.com/"},
		[]string{"//other.com", "/sub", "http://other.com/sub"},
		[]string{"//other.com", "/sub/", "http://other.com/sub/"},

		[]string{"//other.com/", "http://host.com/a", "http://host.com/a"},
		[]string{"//other.com/", "//host.com", "http://host.com"},
		[]string{"//other.com/", "//host.com/", "http://host.com/"},
		[]string{"//other.com/", "//host.com/b", "http://host.com/b"},
		[]string{"//other.com/", "/", "http://other.com/"},
		[]string{"//other.com/", "/sub", "http://other.com/sub"},
		[]string{"//other.com/", "/sub/", "http://other.com/sub/"},

		[]string{"/", "http://host.com/a", "http://host.com/a"},
		[]string{"/", "//host.com", "http://host.com"},
		[]string{"/", "//host.com/", "http://host.com/"},
		[]string{"/", "//host.com/b", "http://host.com/b"},
		[]string{"/", "/", "http://example.com/"},
		[]string{"/", "/sub", "http://example.com/sub"},
		[]string{"/", "/sub/", "http://example.com/sub/"},

		[]string{"/sub", "http://host.com/a", "http://host.com/a"},
		[]string{"/sub", "//host.com", "http://host.com"},
		[]string{"/sub", "//host.com/", "http://host.com/"},
		[]string{"/sub", "//host.com/b", "http://host.com/b"},
		[]string{"/sub", "/", "http://example.com/"}, // 39
		[]string{"/sub", "/sub", "http://example.com/sub"},
		[]string{"/sub", "/sub/", "http://example.com/sub/"},

		[]string{"/sub/", "http://host.com/a", "http://host.com/a"},
		[]string{"/sub/", "//host.com", "http://host.com"},
		[]string{"/sub/", "//host.com/", "http://host.com/"},
		[]string{"/sub/", "//host.com/b", "http://host.com/b"},
		[]string{"/sub/", "/", "http://example.com/"},
		[]string{"/sub/", "/sub", "http://example.com/sub"},
		[]string{"/sub/", "/sub/", "http://example.com/sub/"}, // 48

		[]string{"/sub/index.html", "http://host.com/a", "http://host.com/a"},
		[]string{"/sub/index.html", "//host.com", "http://host.com"},
		[]string{"/sub/index.html", "//host.com/", "http://host.com/"},
		[]string{"/sub/index.html", "//host.com/b", "http://host.com/b"},
		[]string{"/sub/index.html", "/", "http://example.com/"}, // 53
		[]string{"/sub/index.html", "/sub", "http://example.com/sub"},
		[]string{"/sub/index.html", "/sub/", "http://example.com/sub/"},

		[]string{"/sub/", "#top-of-page", "http://example.com/sub/#top-of-page"},
		[]string{"/sub/", "?print=1", "http://example.com/sub/?print=1"},
	}

	for i, c := range testCases {
		if got := handleBaseTag(rootURL, c[0], c[1]); got != c[2] {
			t.Errorf("%d: want %s, got %s", i, c[2], got)
		}
	}
}
