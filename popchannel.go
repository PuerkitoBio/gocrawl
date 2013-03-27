package gocrawl

// The pop channel is a stacked channel used by workers to pop the next URL(s)
// to process.
type popChannel chan []*URLContext

// Constructor to create and initialize a popChannel
func newPopChannel() popChannel {
	// The pop channel is stacked, so only a buffer of 1 is required
	// see http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels
	return make(chan []*URLContext, 1)
}

// The stack function ensures the specified URLs are added to the pop channel
// with minimal blocking (since the channel is stacked, it is virtually equivalent
// to an infinitely buffered channel).
func (this popChannel) stack(cmd ...*URLContext) {
	toStack := cmd
	for {
		select {
		case this <- toStack:
			return
		case old := <-this:
			// Content of the channel got emptied and is now in old, so append whatever
			// is in toStack to it, so that it can either be inserted in the channel,
			// or appended to some other content that got through in the meantime.
			toStack = append(old, toStack...)
		}
	}
}
