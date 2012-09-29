package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"net/url"
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
			// Allow all, unless already visited
			return !isVisited
		} else if len(whitelist) > 0 {
			if indexInStrings(whitelist, target.String()) >= 0 {
				// Allow if whitelisted and not already visited
				return !isVisited
			}
		}
		return false
	}
	return spy
}

func runFileFetcherWithOptions(opts *Options, urlSel []string, seeds []string) (spyv *visitorSpy, spyu *urlSelectorSpy, b *bytes.Buffer) {
	// Initialize log, spies and crawler
	b = new(bytes.Buffer)
	spyv = newVisitorSpy(0, nil, true)
	spyu = newUrlSelectorSpy(0, urlSel...)

	opts.UrlVisitor = spyv.f
	opts.UrlSelector = spyu.f
	opts.Fetcher = newFileFetcher("./testdata/")
	opts.Logger = log.New(b, "", 0)
	c := NewCrawlerWithOptions(opts)

	c.Run(seeds...)
	return spyv, spyu, b
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
