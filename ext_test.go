package gocrawl

import (
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
