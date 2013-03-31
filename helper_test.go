package gocrawl

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const (
	DefaultTestCrawlDelay = 100 * time.Millisecond
)

/*
func newSpyExtenderConfigured(visitDelay time.Duration, returnUrls []*url.URL, doLinks bool,
	filterDelay time.Duration, filterWhitelist ...string) *spyExtender {

	v := func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
		time.Sleep(visitDelay)
		return returnUrls, doLinks
	}

	f := func(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode) {
		time.Sleep(filterDelay)
		if len(filterWhitelist) == 1 && filterWhitelist[0] == "*" {
			// Allow all, unless already visited
			return !isVisited, 0, HrmDefault
		} else if len(filterWhitelist) > 0 {
			if indexInStrings(filterWhitelist, target.String()) >= 0 {
				// Allow if whitelisted and not already visited
				return !isVisited, 0, HrmDefault
			}
		}
		return false, 0, HrmDefault
	}
	return newSpyExtender(v, f)
}

func runFileFetcherWithOptions(opts *Options, urlSel []string, seeds []string) (spy *spyExtender) {
	spy = newSpyExtenderConfigured(0, nil, true, 0, urlSel...)

	opts.Extender = spy
	c := NewCrawlerWithOptions(opts)

	c.Run(seeds...)
	return spy
}
*/

func assertIsInLog(buf bytes.Buffer, s string, t *testing.T) {
	assertLog(buf, s, true, t)
}

func assertIsNotInLog(buf bytes.Buffer, s string, t *testing.T) {
	assertLog(buf, s, false, t)
}

func assertLog(buf bytes.Buffer, s string, in bool, t *testing.T) {
	if lg := buf.String(); strings.Contains(lg, s) != in {
		if in {
			t.Errorf("Expected log to contain %s.", s)
		} else {
			t.Errorf("Expected log NOT to contain %s.", s)
		}
		t.Logf("Log is: %s", lg)
	}
}

func assertCallCount(spy callCounter, key extensionMethodKey, i int64, t *testing.T) {
	cnt := spy.getCallCount(key)
	if cnt != i {
		t.Errorf("Expected %d call count, got %d.", i, cnt)
	}
}

func assertPanic(t *testing.T) {
	if e := recover(); e == nil {
		t.Error("Expected a panic.")
	}
}
