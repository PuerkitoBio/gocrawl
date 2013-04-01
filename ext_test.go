package gocrawl

/*
import (
	"bytes"
	"github.com/PuerkitoBio/purell"
	"net/url"
	"strings"
	"testing"
)

func TestRedirectFilterOut(t *testing.T) {
	spy := newSpy(new(DefaultExtender), true)
	spy.setExtensionMethod(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		// Accept only src.ca
		return !isVisited && u.Host == "src.ca", 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.LogFlags = LogAll
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://src.ca")

	assertCallCount(spy, eMKFilter, 2, t)   // src.ca and radio-canada.ca
	assertCallCount(spy, eMKEnqueued, 2, t) // src.ca and robots.txt
	assertCallCount(spy, eMKFetch, 2, t)    // src.ca and robots.txt
	assertCallCount(spy, eMKVisit, 0, t)    // src.ca redirects, so no visit
}

func TestRedirectFollow(t *testing.T) {
	spy := newSpy(new(DefaultExtender), true)
	spy.setExtensionMethod(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		return !isVisited && from == nil, 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.LogFlags = LogInfo | LogError
	opts.URLNormalizationFlags = purell.FlagsAllGreedy ^ purell.FlagRemoveWWW // Do not remove www, as it redirects to www!
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.SameHostOnly = false
	c := NewCrawlerWithOptions(opts)
	c.Run("http://src.ca")

	assertCallCount(spy, eMKEnqueued, 4, t) // src.ca, radio-canada.ca and both robots.txt
	assertCallCount(spy, eMKFetch, 4, t)    // src.ca, radio-canada.ca and both robots.txt
	assertCallCount(spy, eMKVisit, 1, t)    // src.ca redirects, radio-canada.ca visited
}

func TestRedirectFollowHeadFirst(t *testing.T) {
	spy := newSpy(new(DefaultExtender), true)
	spy.setExtensionMethod(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		return !isVisited && from == nil, 0, HrmDefault
	})

	opts := NewOptions(spy)
	opts.LogFlags = LogInfo | LogError
	opts.HeadBeforeGet = true
	opts.URLNormalizationFlags = purell.FlagsAllGreedy ^ purell.FlagRemoveWWW // Do not remove www, as it redirects to www!
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.SameHostOnly = false
	c := NewCrawlerWithOptions(opts)
	c.Run("http://src.ca")

	assertCallCount(spy, eMKEnqueued, 4, t)   // src.ca, radio-canada.ca and both robots.txt
	assertCallCount(spy, eMKRequestGet, 1, t) // radio-canada.ca only (no HEAD for robots, and src.ca gets redirected)
	assertCallCount(spy, eMKFetch, 5, t)      // src.ca, 2*radio-canada.ca and both robots.txt
	assertCallCount(spy, eMKVisit, 1, t)      // src.ca redirects, radio-canada.ca visited
}
*/
