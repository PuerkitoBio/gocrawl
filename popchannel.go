package gocrawl

import (
	"net/url"
)

type popChannel chan []*url.URL

func newPopChannel() popChannel {
	return make(chan []*url.URL, 1)
}

func (this popChannel) stack(u ...*url.URL) {
	for {
		select {
		case this <- u:
			return
		case old := <-this:
			u = append(u, old...)
		}
	}
}

func (this popChannel) get() (u *url.URL, ok bool) {
	var ar []*url.URL

	if ar, ok = <-this; ok {
		// Impossible that the array is empty
		u = ar[0]
		if len(ar) > 1 {
			// Re-stack only if more urls to process
			this.stack(ar[1:]...)
		}
	}
	return u, ok
}
