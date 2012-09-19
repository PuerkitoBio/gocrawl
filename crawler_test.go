package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	c := NewCrawler("http://provok.in") //("http://www.cyberpresse.ca", "http://www.radio-canada.ca") //, "http://google.ca")
	c.MaxVisits = 2
	c.MaxGoroutines = 1
	c.Run(func(r *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(200 * time.Millisecond)
		return nil, true
	})
}

func TestLevels(t *testing.T) {
	t.Logf("LogNone=%d", LogNone)
	t.Logf("LogError=%d", LogError)
	t.Logf("LogInfo=%d", LogInfo)
	t.Logf("LogTrace=%d", LogTrace)
	t.Logf("LogTrace|LogError=%d", LogTrace|LogError)
}
