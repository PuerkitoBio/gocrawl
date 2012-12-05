package gocrawl

import (
	"bytes"
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
