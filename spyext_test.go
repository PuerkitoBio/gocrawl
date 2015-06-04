package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"reflect"
	"sync"
	"time"
)

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

var (
	lookupEmk = [...]string{
		eMKStart:         "Start",
		eMKEnd:           "End",
		eMKError:         "Error",
		eMKComputeDelay:  "ComputeDelay",
		eMKFetch:         "Fetch",
		eMKRequestRobots: "RequestRobots",
		eMKRequestGet:    "RequestGet",
		eMKFetchedRobots: "FetchedRobots",
		eMKFilter:        "Filter",
		eMKEnqueued:      "Enqueued",
		eMKVisit:         "Visit",
		eMKVisited:       "Visited",
		eMKDisallowed:    "Disallowed",
	}
)

// Type and unique value to identify a called-with argument position to ignore.
type ignoredCalledWithArg int

const __ ignoredCalledWithArg = -1

func (this extensionMethodKey) String() string {
	return lookupEmk[this]
}

// The spy extender adds counting the number of calls for each extender method,
// and tracks the log so that it can be asserted.
type spyExtender struct {
	Extender
	useLogBuffer bool
	callCount    map[extensionMethodKey]int
	methods      map[extensionMethodKey]interface{}
	calledWith   map[extensionMethodKey][][]interface{}
	b            bytes.Buffer
	m            sync.RWMutex       // Protects access to call count, methods and called with maps
	logM         sync.Mutex         // Protects access to the log buffer (b)
	EnqueueChan  chan<- interface{} // Redefine here, not accessible on DefaultExtender
}

func newSpy(ext Extender, useLogBuffer bool) *spyExtender {
	return &spyExtender{
		ext,
		useLogBuffer,
		make(map[extensionMethodKey]int, eMKLast),
		make(map[extensionMethodKey]interface{}, 2),
		make(map[extensionMethodKey][][]interface{}, eMKLast),
		bytes.Buffer{},
		sync.RWMutex{},
		sync.Mutex{},
		nil,
	}
}

func (this *spyExtender) setExtensionMethod(key extensionMethodKey, f interface{}) {
	this.m.Lock()
	defer this.m.Unlock()
	this.methods[key] = f
}

func (this *spyExtender) getCallCount(key extensionMethodKey) int {
	this.m.RLock()
	defer this.m.RUnlock()
	return this.callCount[key]
}

func (this *spyExtender) registerCall(key extensionMethodKey, args ...interface{}) {
	this.m.Lock()
	defer this.m.Unlock()

	// Increment call count
	this.callCount[key] += 1

	// Register called-with arguments
	data, _ := this.calledWith[key]
	data = append(data, args)
	this.calledWith[key] = data
}

func (this *spyExtender) getCalledWithCount(key extensionMethodKey, args ...interface{}) int {
	this.m.RLock()
	defer this.m.RUnlock()
	calls := this.calledWith[key]
	cnt := 0
	for _, calledArgs := range calls {
		if isCalledWith(calledArgs, args) {
			cnt++
		}
	}
	return cnt
}

func isCalledWith(actual, compare []interface{}) bool {
	if len(actual) != len(compare) {
		return false
	}
	for i, v := range actual {
		if compare[i] == __ {
			continue
		}
		if !reflect.DeepEqual(v, compare[i]) {
			return false
		}
	}
	return true
}

func (this *spyExtender) Log(logFlags LogFlags, msgLevel LogFlags, msg string) {
	if this.useLogBuffer {
		if logFlags&msgLevel == msgLevel {
			this.logM.Lock()
			defer this.logM.Unlock()
			this.b.WriteString(msg + "\n")
		}
	} else {
		this.Extender.Log(logFlags, msgLevel, msg)
	}
}

func (this *spyExtender) Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
	this.registerCall(eMKVisit, ctx, res, doc)
	if f, ok := this.methods[eMKVisit].(func(*URLContext, *http.Response, *goquery.Document) (interface{}, bool)); ok {
		return f(ctx, res, doc)
	}
	return this.Extender.Visit(ctx, res, doc)
}

func (this *spyExtender) Filter(ctx *URLContext, isVisited bool) bool {
	this.registerCall(eMKFilter, ctx, isVisited)
	if f, ok := this.methods[eMKFilter].(func(*URLContext, bool) bool); ok {
		return f(ctx, isVisited)
	}
	return this.Extender.Filter(ctx, isVisited)
}

func (this *spyExtender) Start(seeds interface{}) interface{} {
	this.registerCall(eMKStart, seeds)
	if f, ok := this.methods[eMKStart].(func(interface{}) interface{}); ok {
		return f(seeds)
	}
	return this.Extender.Start(seeds)
}

func (this *spyExtender) End(err error) {
	this.registerCall(eMKEnd, err)
	if f, ok := this.methods[eMKEnd].(func(error)); ok {
		f(err)
		return
	}
	this.Extender.End(err)
}

func (this *spyExtender) Error(err *CrawlError) {
	this.registerCall(eMKError, err)
	if f, ok := this.methods[eMKError].(func(*CrawlError)); ok {
		f(err)
		return
	}
	this.Extender.Error(err)
}

func (this *spyExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	this.registerCall(eMKComputeDelay, host, di, lastFetch)
	if f, ok := this.methods[eMKComputeDelay].(func(string, *DelayInfo, *FetchInfo) time.Duration); ok {
		return f(host, di, lastFetch)
	}
	return this.Extender.ComputeDelay(host, di, lastFetch)
}

func (this *spyExtender) Fetch(ctx *URLContext, userAgent string, headRequest bool) (*http.Response, error) {
	this.registerCall(eMKFetch, ctx, userAgent, headRequest)
	if f, ok := this.methods[eMKFetch].(func(*URLContext, string, bool) (*http.Response, error)); ok {
		return f(ctx, userAgent, headRequest)
	}
	return this.Extender.Fetch(ctx, userAgent, headRequest)
}

func (this *spyExtender) RequestRobots(ctx *URLContext, robotAgent string) (data []byte, doRequest bool) {
	this.registerCall(eMKRequestRobots, ctx, robotAgent)
	if f, ok := this.methods[eMKRequestRobots].(func(*URLContext, string) ([]byte, bool)); ok {
		return f(ctx, robotAgent)
	}
	return this.Extender.RequestRobots(ctx, robotAgent)
}

func (this *spyExtender) RequestGet(ctx *URLContext, headRes *http.Response) bool {
	this.registerCall(eMKRequestGet, ctx, headRes)
	if f, ok := this.methods[eMKRequestGet].(func(*URLContext, *http.Response) bool); ok {
		return f(ctx, headRes)
	}
	return this.Extender.RequestGet(ctx, headRes)
}

func (this *spyExtender) FetchedRobots(ctx *URLContext, res *http.Response) {
	this.registerCall(eMKFetchedRobots, ctx, res)
	if f, ok := this.methods[eMKFetchedRobots].(func(*URLContext, *http.Response)); ok {
		f(ctx, res)
		return
	}
	this.Extender.FetchedRobots(ctx, res)
}

func (this *spyExtender) Enqueued(ctx *URLContext) {
	this.registerCall(eMKEnqueued, ctx)
	if f, ok := this.methods[eMKEnqueued].(func(*URLContext)); ok {
		f(ctx)
		return
	}
	this.Extender.Enqueued(ctx)
}

func (this *spyExtender) Visited(ctx *URLContext, harvested interface{}) {
	this.registerCall(eMKVisited, ctx, harvested)
	if f, ok := this.methods[eMKVisited].(func(*URLContext, interface{})); ok {
		f(ctx, harvested)
		return
	}
	this.Extender.Visited(ctx, harvested)
}

func (this *spyExtender) Disallowed(ctx *URLContext) {
	this.registerCall(eMKDisallowed, ctx)
	if f, ok := this.methods[eMKDisallowed].(func(*URLContext)); ok {
		f(ctx)
		return
	}
	this.Extender.Disallowed(ctx)
}
