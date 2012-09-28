package gocrawl

import (
	"bytes"
	"log"
	"testing"
	"time"
)

func runFileFetcherWithOptions(opts *Options, urlSel []string, seeds []string) (spyv *visitorSpy, spyu *urlSelectorSpy, b *bytes.Buffer) {
	// Initialize log, spies and crawler
	b = new(bytes.Buffer)
	spyv = newVisitorSpy(0, nil, true)
	spyu = newUrlSelectorSpy(0, urlSel...)

	opts.UrlVisitor = spyv.f
	opts.UrlSelector = spyu.f
	opts.Fetcher = newFileFetcher("./testdata/")
	opts.Logger = log.New(b, "", 0)
	c := NewCrawlerWithOptions(opts)

	c.Run(seeds...)
	return spyv, spyu, b
}

func TestAllSameHost(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = true
	opts.CrawlDelay = 1 * time.Second
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spyv, 5, t)
	assertCallCount(spyu, 13, t)
}

func TestAllNotSameHost(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts, []string{"*"}, []string{"http://hosta/page1.html", "http://hosta/page4.html"})

	assertCallCount(spyv, 10, t)
	assertCallCount(spyu, 24, t)
}

func TestSelectOnlyPage1s(t *testing.T) {
	opts := NewOptions(nil, nil)
	opts.SameHostOnly = false
	opts.CrawlDelay = 1 * time.Second
	opts.LogFlags = LogError | LogTrace
	spyv, spyu, _ := runFileFetcherWithOptions(opts,
		[]string{"http://hosta/page1.html", "http://hostb/page1.html", "http://hostc/page1.html", "http://hostd/page1.html"},
		[]string{"http://hosta/page1.html", "http://hosta/page4.html", "http://hostb/pageunlinked.html"})

	assertCallCount(spyv, 3, t)
	assertCallCount(spyu, 11, t)
}
