package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	c := NewCrawler("http://provok.in") //, "http://google.ca")
	c.Run(func(r *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(200 * time.Millisecond)
		return nil, true
	})
}
