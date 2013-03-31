package gocrawl

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"net/http"
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

func (this *spyExtender) Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (harvested interface{}, findLinks bool) {
	this.incCallCount(eMKVisit, 1)
	if f, ok := this.methods[eMKVisit].(func(*URLContext, *http.Response, *goquery.Document) (interface{}, bool)); ok {
		return f(ctx, res, doc)
	}
	return this.ext.Visit(ctx, res, doc)
}

func (this *spyExtender) Filter(ctx *URLContext, isVisited bool) bool {
	this.incCallCount(eMKFilter, 1)
	if f, ok := this.methods[eMKFilter].(func(*URLContext, bool) bool); ok {
		return f(ctx, isVisited)
	}
	return this.ext.Filter(ctx, isVisited)
}

func (this *spyExtender) Start(seeds interface{}) interface{} {
	this.incCallCount(eMKStart, 1)
	if f, ok := this.methods[eMKStart].(func(interface{}) interface{}); ok {
		return f(seeds)
	}
	return this.ext.Start(seeds)
}

func (this *spyExtender) End(err error) {
	this.incCallCount(eMKEnd, 1)
	if f, ok := this.methods[eMKEnd].(func(error)); ok {
		f(err)
		return
	}
	this.ext.End(err)
}

func (this *spyExtender) Error(err *CrawlError) {
	this.incCallCount(eMKError, 1)
	if f, ok := this.methods[eMKError].(func(*CrawlError)); ok {
		f(err)
		return
	}
	this.ext.Error(err)
}

func (this *spyExtender) ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration {
	this.incCallCount(eMKComputeDelay, 1)
	if f, ok := this.methods[eMKComputeDelay].(func(string, *DelayInfo, *FetchInfo) time.Duration); ok {
		return f(host, di, lastFetch)
	}
	return this.ext.ComputeDelay(host, di, lastFetch)
}

func (this *spyExtender) Fetch(ctx *URLContext, userAgent string, headRequest bool) (*http.Response, error) {
	this.incCallCount(eMKFetch, 1)
	if f, ok := this.methods[eMKFetch].(func(*URLContext, string, bool) (*http.Response, error)); ok {
		return f(ctx, userAgent, headRequest)
	}
	return this.ext.Fetch(ctx, userAgent, headRequest)
}

func (this *spyExtender) RequestRobots(ctx *URLContext, robotAgent string) (data []byte, doRequest bool) {
	this.incCallCount(eMKRequestRobots, 1)
	if f, ok := this.methods[eMKRequestRobots].(func(*URLContext, string) ([]byte, bool)); ok {
		return f(ctx, robotAgent)
	}
	return this.ext.RequestRobots(ctx, robotAgent)
}

func (this *spyExtender) RequestGet(ctx *URLContext, headRes *http.Response) bool {
	this.incCallCount(eMKRequestGet, 1)
	if f, ok := this.methods[eMKRequestGet].(func(*URLContext, *http.Response) bool); ok {
		return f(ctx, headRes)
	}
	return this.ext.RequestGet(ctx, headRes)
}

func (this *spyExtender) FetchedRobots(ctx *URLContext, res *http.Response) {
	this.incCallCount(eMKFetchedRobots, 1)
	if f, ok := this.methods[eMKFetchedRobots].(func(*URLContext, *http.Response)); ok {
		f(ctx, res)
		return
	}
	this.ext.FetchedRobots(ctx, res)
}

func (this *spyExtender) Enqueued(ctx *URLContext) {
	this.incCallCount(eMKEnqueued, 1)
	if f, ok := this.methods[eMKEnqueued].(func(*URLContext)); ok {
		f(ctx)
		return
	}
	this.ext.Enqueued(ctx)
}

func (this *spyExtender) Visited(ctx *URLContext, harvested interface{}) {
	this.incCallCount(eMKVisited, 1)
	if f, ok := this.methods[eMKVisited].(func(*URLContext, interface{})); ok {
		f(ctx, harvested)
		return
	}
	this.ext.Visited(ctx, harvested)
}

func (this *spyExtender) Disallowed(ctx *URLContext) {
	this.incCallCount(eMKDisallowed, 1)
	if f, ok := this.methods[eMKDisallowed].(func(*URLContext)); ok {
		f(ctx)
		return
	}
	this.ext.Disallowed(ctx)
}
