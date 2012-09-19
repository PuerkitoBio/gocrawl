package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	c := NewCrawler("http://www.cyberpresse.ca", "http://www.radio-canada.ca") //, "http://google.ca")
	c.MaxVisits = 20
	c.Run(func(r *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(200 * time.Millisecond)
		return nil, true
	})
}
