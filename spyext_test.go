package gocrawl

// Extension method enum
type extensionMethodKey uint8

const (
	eMKStart extensionMethodKey = iota
	eMKEnd
	eMKError
	eMKComputeDelay
	eMKFetch
	eMKRequestRobots
	eMKRequestGet
	eMKFetchedRobots
	eMKFilter
	eMKEnqueued
	eMKVisit
	eMKVisited
	eMKDisallowed
	eMKLast
)

// callCounter interface implemented by the Spy Extender.
type callCounter interface {
	getCallCount(extensionMethodKey) int64
	incCallCount(extensionMethodKey, int64)
}

// The spy extender adds counting the number of calls for each extender method,
// and tracks the log so that it can be asserted.
type spyExtender struct {
	ext          Extender
	useLogBuffer bool
	callCount    map[extensionMethodKey]int64
	methods      map[extensionMethodKey]interface{}
	b            bytes.Buffer
	// Since EnqueueChan is unaccessible for GoCrawl to set, add it as an explicit field
	EnqueueChan chan<- interface{}
	m           sync.RWMutex
}

func newSpy(ext Extender, useLogBuffer bool) *spyExtender {
	return &spyExtender{ext: ext,
		useLogBuffer: useLogBuffer,
		callCount:    make(map[extensionMethodKey]int64, eMKLast),
		methods:      make(map[extensionMethodKey]interface{}, 2),
	}
}

func (this *spyExtender) setExtensionMethod(key extensionMethodKey, f interface{}) {
	this.m.Lock()
	defer this.m.Unlock()
	this.methods[key] = f
}

func (this *spyExtender) getCallCount(key extensionMethodKey) int64 {
	this.m.RLock()
	defer this.m.RUnlock()
	return this.callCount[key]
}

func (this *spyExtender) incCallCount(key extensionMethodKey, delta int64) {
	this.m.Lock()
	defer this.m.Unlock()
	this.callCount[key] += delta
}

func (this *spyExtender) Log(logFlags LogFlags, msgLevel LogFlags, msg string) {
	if this.useLogBuffer {
		if logFlags&msgLevel == msgLevel {
			this.b.WriteString(msg + "\n")
		}
	} else {
		this.ext.Log(logFlags, msgLevel, msg)
	}
}

func (this *spyExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
	this.incCallCount(eMKVisit, 1)
	if f, ok := this.methods[eMKVisit].(func(res *http.Response, doc *goquery.Document) ([]*url.URL, bool)); ok {
		return f(res, doc)
	}
	return this.ext.Visit(res, doc)
}

func (this *spyExtender) Filter(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode) {
	this.incCallCount(eMKFilter, 1)
	if f, ok := this.methods[eMKFilter].(func(target *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode)); ok {
		return f(target, from, isVisited, origin)
	}
	return this.ext.Filter(target, from, isVisited, origin)
}

func (this *spyExtender) Start(seeds []string) []string {
	this.incCallCount(eMKStart, 1)
	if f, ok := this.methods[eMKStart].(func(seeds []string) []string); ok {
		return f(seeds)
	}
	return this.ext.Start(seeds)
}

func (this *spyExtender) End(reason EndReason) {
	this.incCallCount(eMKEnd, 1)
	if f, ok := this.methods[eMKEnd].(func(reason EndReason)); ok {
		f(reason)
		return
	}
	this.ext.End(reason)
}

func (this *spyExtender) Error(err *CrawlError) {
	this.incCallCount(eMKError, 1)
	if f, ok := this.methods[eMKError].(func(err *CrawlError)); ok {
		f(err)
		return
	}
	this.ext.Error(err)
}

func (this *spyExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	this.incCallCount(eMKComputeDelay, 1)
	if f, ok := this.methods[eMKComputeDelay].(func(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration); ok {
		return f(host, di, lastFetch)
	}
	return this.ext.ComputeDelay(host, di, lastFetch)
}

func (this *spyExtender) Fetch(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error) {
	this.incCallCount(eMKFetch, 1)
	if f, ok := this.methods[eMKFetch].(func(u *url.URL, userAgent string, headRequest bool) (res *http.Response, err error)); ok {
		return f(u, userAgent, headRequest)
	}
	return this.ext.Fetch(u, userAgent, headRequest)
}

func (this *spyExtender) RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte) {
	this.incCallCount(eMKRequestRobots, 1)
	if f, ok := this.methods[eMKRequestRobots].(func(u *url.URL, robotAgent string) (request bool, data []byte)); ok {
		return f(u, robotAgent)
	}
	return this.ext.RequestRobots(u, robotAgent)
}

func (this *spyExtender) RequestGet(headRes *http.Response) bool {
	this.incCallCount(eMKRequestGet, 1)
	if f, ok := this.methods[eMKRequestGet].(func(headRes *http.Response) bool); ok {
		return f(headRes)
	}
	return this.ext.RequestGet(headRes)
}

func (this *spyExtender) FetchedRobots(res *http.Response) {
	this.incCallCount(eMKFetchedRobots, 1)
	if f, ok := this.methods[eMKFetchedRobots].(func(res *http.Response)); ok {
		f(res)
		return
	}
	this.ext.FetchedRobots(res)
}

func (this *spyExtender) Enqueued(u *url.URL, from *url.URL) {
	this.incCallCount(eMKEnqueued, 1)
	if f, ok := this.methods[eMKEnqueued].(func(u *url.URL, from *url.URL)); ok {
		f(u, from)
		return
	}
	this.ext.Enqueued(u, from)
}

func (this *spyExtender) Visited(u *url.URL, harvested []*url.URL) {
	this.incCallCount(eMKVisited, 1)
	if f, ok := this.methods[eMKVisited].(func(u *url.URL, harvested []*url.URL)); ok {
		f(u, harvested)
		return
	}
	this.ext.Visited(u, harvested)
}

func (this *spyExtender) Disallowed(u *url.URL) {
	this.incCallCount(eMKDisallowed, 1)
	if f, ok := this.methods[eMKDisallowed].(func(u *url.URL)); ok {
		f(u)
		return
	}
	this.ext.Disallowed(u)
}
