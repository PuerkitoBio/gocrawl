package gocrawl

import (
	"net/url"
)

type popChannel chan []*url.URL

func newPopChannel() popChannel {
	// The pop channel is stacked, so only a buffer of 1 is required
	// see http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels
	return make(chan []*url.URL, 1)
}

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
