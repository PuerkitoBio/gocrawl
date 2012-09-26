package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type callCounter interface {
	getCallCount() int64
}

type callCounterImpl struct {
	callCount int64
}

func (this *callCounterImpl) getCallCount() int64 {
	return this.callCount
}

type visitorSpy struct {
	callCounterImpl
	f func(*http.Response, *goquery.Document) ([]*url.URL, bool)
}

func newVisitorSpy(delay time.Duration, urls []*url.URL, ret bool) *visitorSpy {
	spy := new(visitorSpy)
	spy.f = func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		spy.callCounterImpl.callCount = atomic.AddInt64(&spy.callCounterImpl.callCount, 1)
		time.Sleep(delay)
		return urls, ret
	}
	return spy
}

type urlSelectorSpy struct {
	callCounterImpl
	f func(*url.URL, *url.URL, bool) bool
}

func newUrlSelectorSpy(delay time.Duration, whitelist ...string) *urlSelectorSpy {
	spy := new(urlSelectorSpy)
	spy.f = func(target *url.URL, origin *url.URL, isVisited bool) bool {
		spy.callCounterImpl.callCount = atomic.AddInt64(&spy.callCounterImpl.callCount, 1)
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
	return spy
}

func assertIsInLog(buf bytes.Buffer, s string, t *testing.T) {
	if lg := buf.String(); !strings.Contains(lg, s) {
		t.Errorf("Expected log to contain %s.", s)
		t.Logf("Log is: %s", lg)
	}
}

func assertCallCount(cc callCounter, i int64, t *testing.T) {
	if n := cc.getCallCount(); n != i {
		t.Errorf("Expected %d call count, got %d.", i, n)
	}
}

func xTestBasic(t *testing.T) {
	spy := newVisitorSpy(1*time.Millisecond, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.MaxVisits = 5
	c.Options.SameHostOnly = false
	c.Options.LogFlags = LogError | LogTrace

	c.Run("http://provok.in")
}

func TestInvalidSeed(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(1*time.Millisecond, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	c.Run("#toto")
	assertIsInLog(b, "Error parsing seed URL", t)
	assertCallCount(spy, 0, t)
}

func TestHostCount(t *testing.T) {
	var b bytes.Buffer

	spy := newVisitorSpy(1*time.Millisecond, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)

	// Use ftp scheme so that it doesn't actually attempt a fetch
	c.Run("ftp://roota/a", "ftp://roota/b", "ftp://rootb/c")
	assertIsInLog(b, "Initial host count is 2", t)
	assertIsInLog(b, "Parsed seeds length: 3", t)
	assertCallCount(spy, 0, t)
}

func TestCustomSelectorNoUrl(t *testing.T) {
	var b bytes.Buffer

	vspy := newVisitorSpy(1*time.Millisecond, nil, true)
	uspy := newUrlSelectorSpy(0)
	c := NewCrawler(vspy.f, uspy.f)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Options.Logger = log.New(&b, "", 0)
	c.Run("http://test1", "http://test2")
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test1", t)
	assertIsInLog(b, "Ignore URL on Custom Selector policy http://test2", t)
	assertCallCount(uspy, 2, t)
	assertCallCount(vspy, 0, t)
}

func TestNoSeed(t *testing.T) {
	spy := newVisitorSpy(0, nil, true)
	c := NewCrawler(spy.f, nil)

	c.Options.CrawlDelay = 1 * time.Second
	c.Options.LogFlags = LogError | LogTrace
	c.Run()
	assertCallCount(spy, 0, t)
}

// TODO : Use a Fetcher with static data for tests
