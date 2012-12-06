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
