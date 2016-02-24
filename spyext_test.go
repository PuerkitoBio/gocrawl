package gocrawl

import (
	"bytes"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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

const ignore ignoredCalledWithArg = -1

func (k extensionMethodKey) String() string {
	return lookupEmk[k]
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

func (x *spyExtender) setExtensionMethod(key extensionMethodKey, f interface{}) {
	x.m.Lock()
	defer x.m.Unlock()
	x.methods[key] = f
}

func (x *spyExtender) getCallCount(key extensionMethodKey) int {
	x.m.RLock()
	defer x.m.RUnlock()
	return x.callCount[key]
}

func (x *spyExtender) registerCall(key extensionMethodKey, args ...interface{}) {
	x.m.Lock()
	defer x.m.Unlock()

	// Increment call count
	x.callCount[key]++

	// Register called-with arguments
	data, _ := x.calledWith[key]
	data = append(data, args)
	x.calledWith[key] = data
}

func (x *spyExtender) getCalledWithCount(key extensionMethodKey, args ...interface{}) int {
	x.m.RLock()
	defer x.m.RUnlock()
	calls := x.calledWith[key]
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
		if compare[i] == ignore {
			continue
		}
		if !reflect.DeepEqual(v, compare[i]) {
			return false
		}
	}
	return true
}

func (x *spyExtender) Log(logFlags LogFlags, msgLevel LogFlags, msg string) {
	if x.useLogBuffer {
		if logFlags&msgLevel == msgLevel {
			x.logM.Lock()
			defer x.logM.Unlock()
			x.b.WriteString(msg + "\n")
		}
	} else {
		x.Extender.Log(logFlags, msgLevel, msg)
	}
}

func (x *spyExtender) Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
	x.registerCall(eMKVisit, ctx, res, doc)
	if f, ok := x.methods[eMKVisit].(func(*URLContext, *http.Response, *goquery.Document) (interface{}, bool)); ok {
		return f(ctx, res, doc)
	}
	return x.Extender.Visit(ctx, res, doc)
}

func (x *spyExtender) Filter(ctx *URLContext, isVisited bool) bool {
	x.registerCall(eMKFilter, ctx, isVisited)
	if f, ok := x.methods[eMKFilter].(func(*URLContext, bool) bool); ok {
		return f(ctx, isVisited)
	}
	return x.Extender.Filter(ctx, isVisited)
}

func (x *spyExtender) Start(seeds interface{}) interface{} {
	x.registerCall(eMKStart, seeds)
	if f, ok := x.methods[eMKStart].(func(interface{}) interface{}); ok {
		return f(seeds)
	}
	return x.Extender.Start(seeds)
}

func (x *spyExtender) End(err error) {
	x.registerCall(eMKEnd, err)
	if f, ok := x.methods[eMKEnd].(func(error)); ok {
		f(err)
		return
	}
	x.Extender.End(err)
}

func (x *spyExtender) Error(err *CrawlError) {
	x.registerCall(eMKError, err)
	if f, ok := x.methods[eMKError].(func(*CrawlError)); ok {
		f(err)
		return
	}
	x.Extender.Error(err)
}

func (x *spyExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	x.registerCall(eMKComputeDelay, host, di, lastFetch)
	if f, ok := x.methods[eMKComputeDelay].(func(string, *DelayInfo, *FetchInfo) time.Duration); ok {
		return f(host, di, lastFetch)
	}
	return x.Extender.ComputeDelay(host, di, lastFetch)
}

func (x *spyExtender) Fetch(ctx *URLContext, userAgent string, headRequest bool) (*http.Response, error) {
	x.registerCall(eMKFetch, ctx, userAgent, headRequest)
	if f, ok := x.methods[eMKFetch].(func(*URLContext, string, bool) (*http.Response, error)); ok {
		return f(ctx, userAgent, headRequest)
	}
	return x.Extender.Fetch(ctx, userAgent, headRequest)
}

func (x *spyExtender) RequestRobots(ctx *URLContext, robotAgent string) (data []byte, doRequest bool) {
	x.registerCall(eMKRequestRobots, ctx, robotAgent)
	if f, ok := x.methods[eMKRequestRobots].(func(*URLContext, string) ([]byte, bool)); ok {
		return f(ctx, robotAgent)
	}
	return x.Extender.RequestRobots(ctx, robotAgent)
}

func (x *spyExtender) RequestGet(ctx *URLContext, headRes *http.Response) bool {
	x.registerCall(eMKRequestGet, ctx, headRes)
	if f, ok := x.methods[eMKRequestGet].(func(*URLContext, *http.Response) bool); ok {
		return f(ctx, headRes)
	}
	return x.Extender.RequestGet(ctx, headRes)
}

func (x *spyExtender) FetchedRobots(ctx *URLContext, res *http.Response) {
	x.registerCall(eMKFetchedRobots, ctx, res)
	if f, ok := x.methods[eMKFetchedRobots].(func(*URLContext, *http.Response)); ok {
		f(ctx, res)
		return
	}
	x.Extender.FetchedRobots(ctx, res)
}

func (x *spyExtender) Enqueued(ctx *URLContext) {
	x.registerCall(eMKEnqueued, ctx)
	if f, ok := x.methods[eMKEnqueued].(func(*URLContext)); ok {
		f(ctx)
		return
	}
	x.Extender.Enqueued(ctx)
}

func (x *spyExtender) Visited(ctx *URLContext, harvested interface{}) {
	x.registerCall(eMKVisited, ctx, harvested)
	if f, ok := x.methods[eMKVisited].(func(*URLContext, interface{})); ok {
		f(ctx, harvested)
		return
	}
	x.Extender.Visited(ctx, harvested)
}

func (x *spyExtender) Disallowed(ctx *URLContext) {
	x.registerCall(eMKDisallowed, ctx)
	if f, ok := x.methods[eMKDisallowed].(func(*URLContext)); ok {
		f(ctx)
		return
	}
	x.Extender.Disallowed(ctx)
}
