package gocrawl

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

func TestEnqueueChanDefault(t *testing.T) {
	var de = new(DefaultExtender)

	c := NewCrawler(de)
	if de.EnqueueChan != nil {
		t.Fatal("Expected EnqueueChan to be nil")
	}
	c.Run()
	if de.EnqueueChan == nil {
		t.Fatal("Expected EnqueueChan to be non-nil")
	}
}

func TestEnqueueChanEmbedded(t *testing.T) {
	type MyExt struct {
		SomeFieldBefore bool
		*DefaultExtender
		SomeFieldAfter int
	}
	me := &MyExt{false, new(DefaultExtender), 0}

	c := NewCrawler(me)
	if me.EnqueueChan != nil {
		t.Fatal("Expected EnqueueChan to be nil")
	}
	c.Run()
	if me.EnqueueChan == nil {
		t.Fatal("Expected EnqueueChan to be non-nil")
	}
	me.EnqueueChan <- &CrawlerCommand{}
	t.Logf("Chan len = %d", len(me.EnqueueChan))
}

type MyExt struct {
	*DefaultExtender
	EnqueueChan int
	b           *bytes.Buffer
}

func (this *MyExt) Log(logFlags LogFlags, msgLevel LogFlags, msg string) {
	if logFlags&msgLevel == msgLevel {
		this.b.WriteString(msg + "\n")
	}
}

func TestEnqueueChanShadowed(t *testing.T) {
	me := &MyExt{new(DefaultExtender), 0, new(bytes.Buffer)}

	c := NewCrawler(me)
	c.Options.LogFlags = LogInfo
	c.Run()
	assertIsInLog(*me.b, "extender.EnqueueChan is not of type chan<-*gocrawl.CrawlerCommand, cannot set the enqueue channel\n", t)
}

func TestEnqueueNewUrl(t *testing.T) {
	spy := newSpyExtenderFunc(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		// Accept only non-visited Page1s
		return !isVisited && strings.HasSuffix(u.Path, "page1.html"), 0, HrmDefault
	})

	enqueued := false
	spy.setExtensionMethod(eMKEnqueued, func(u *url.URL, from *url.URL) {
		// Add hostc's Page1 to crawl
		if !enqueued {
			newU, _ := url.Parse("http://hostc/page1.html")
			spy.EnqueueChan <- &CrawlerCommand{newU, EoCustomStart}
			enqueued = true
		}
	})

	opts := NewOptions(spy)
	opts.CrawlDelay = DefaultTestCrawlDelay
	opts.LogFlags = LogAll
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hostb/page1.html")

	assertCallCount(spy, eMKFilter, 7, t)
	assertCallCount(spy, eMKEnqueued, 4, t) // robots.txt * 2, both Page1s
}

func TestEnqueueNewUrlOnError(t *testing.T) {
	spy := newSpyExtenderFunc(eMKFilter, func(u *url.URL, from *url.URL, isVisited bool, o EnqueueOrigin) (enqueue bool, priority int, hrm HeadRequestMode) {
		// If is visited, but has an origin of Error, allow
		if isVisited && o == EoError {
			return true, 0, HrmDefault
		}
		// Accept only non-visited
		return !isVisited, 0, HrmDefault
	})

	once := false
	spy.setExtensionMethod(eMKError, func(err *CrawlError) {
		if err.Kind == CekFetch && !once {
			// On error, reenqueue once only
			once = true
			spy.EnqueueChan <- &CrawlerCommand{err.URL, EoError}
		}
	})

	opts := NewOptions(spy)
	opts.LogFlags = LogAll
	opts.CrawlDelay = DefaultTestCrawlDelay
	c := NewCrawlerWithOptions(opts)
	c.Run("http://hosta/page6.html")

	assertCallCount(spy, eMKFilter, 2, t)   // First pass and re-enqueued from error
	assertCallCount(spy, eMKEnqueued, 3, t) // Twice and robots.txt
}

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
