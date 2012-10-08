package gocrawl

import (
	"net/url"
)

// The pop channel is a stacked channel used by workers to pop the next URL(s)
// to process.
type popChannel chan []*url.URL

// Constructor to create and initialize a popChannel
func newPopChannel() popChannel {
	// The pop channel is stacked, so only a buffer of 1 is required
	// see http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels
	return make(chan []*url.URL, 1)
}

// The stack function ensures the specified URLs are added to the pop channel
// with minimal blocking (since the channel is stacked, it is virtually equivalent
// to an infinitely buffered channel).
func (this popChannel) stack(u ...*url.URL) {
	toStack := u
	for {
		select {
		case this <- toStack:
			return
		case old := <-this:
			toStack = append(old, u...)
		}
	}
}
