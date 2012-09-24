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

func (this popChannel) get() (u *url.URL) {
	var ar []*url.URL

	ar = <-this
	// Impossible that the array is empty
	u = ar[0]
	if len(ar) > 1 {
		// Re-stack only if more urls to process
		this.stack(ar[1:]...)
	}

	return u
}
