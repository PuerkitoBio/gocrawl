package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func assertIsInLog(buf bytes.Buffer, s string, t *testing.T) {
	if lg := buf.String(); !strings.Contains(lg, s) {
		t.Errorf("Expected log to contain %s.", s)
		t.Logf("Log is: %s", lg)
	}
}

func getVisitorFunc(delay time.Duration, urls []*url.URL, ret bool) func(*http.Response, *goquery.Document) ([]*url.URL, bool) {
	return func(r *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(delay)
		return urls, ret
	}
}

func getSelectorFunc(delay time.Duration, whitelist ...string) func(*url.URL, *url.URL, bool) bool {
	return func(target *url.URL, origin *url.URL, isVisited bool) bool {
		time.Sleep(delay)
		if len(whitelist) == 1 && whitelist[0] == "*" {
			// Allow all
			return true
		} else if len(whitelist) > 0 {
			if sort.Strings(whitelist); sort.SearchStrings(whitelist, target.String()) < len(whitelist) {
				return true
			}
		}
		return false
	}
}

func TestBasic(t *testing.T) {
	c := NewCrawler(getVisitorFunc(10*time.Millisecond, nil, true), nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.MaxVisits = 5
	c.Options.SameHostOnly = false
	c.Options.LogFlags = LogError | LogTrace

	c.Run("http://provok.in")
}

func TestInvalidSeed(t *testing.T) {
	var b bytes.Buffer

	c := NewCrawler(getVisitorFunc(10*time.Millisecond, nil, true), nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	c.Run("#toto")
	assertIsInLog(b, "Error parsing seed URL", t)
}

func TestHostCount(t *testing.T) {
	var b bytes.Buffer

	c := NewCrawler(getVisitorFunc(10*time.Millisecond, nil, true), nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	// Use ftp scheme so that it doesn't actually attempt a fetch
	c.Run("ftp://roota/a", "ftp://roota/b", "ftp://rootb/c")
	assertIsInLog(b, "Initial host count is 2", t)
	assertIsInLog(b, "Parsed seeds length: 3", t)
}

func TestCustomSelectorNoUrl(t *testing.T) {
	var b bytes.Buffer

	c := NewCrawler(getVisitorFunc(10*time.Millisecond, nil, true),
		getSelectorFunc(0))

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run("http://test1", "http://test2")
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test1", t)
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test2", t)
}

// TODO : Use a Fetcher with static data for tests
