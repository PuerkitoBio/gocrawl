package gocrawl

import (
	"testing"
)

func TestEnqueueChanField(t *testing.T) {
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
