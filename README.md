# gocrawl

gocrawl is a polite, slim and concurrent web crawler written in Go.

## Features

*    Full control over the URLs to visit, inspect and query (using a pre-initialized [goquery][] document)
*    Crawl delays applied per host
*    Obedience to robots.txt rules (using [robotstxt.go][robots] go library)
*    Concurrent execution using goroutines
*    Configurable logging using the builtin Go logger
*    Open, customizable design by providing hooks into the execution logic

gocrawl can be described as a minimalist web crawler, providing the basic engine upon which to build a full-fledged indexing machine with caching, persistence and staleness detection logic, or to use as is for quick and easy crawling. For gocrawl itself does not attempt to detect staleness of a page, nor does it implement a caching mechanism. If an URL is enqueued to be processed, it *will* make a request to fetch it. And there is no prioritization among the URLs to process, it assumes that all enqueued URLs must be visited at some point, and that the order in which they are is unimportant.

However, it does provide a hook (the URLSelector function) where some analysis can take place to alter its behaviour (based on the last visited date of the URL, saved in a persistent store, for example). Instead of trying to do everything and impose a way to do it, it offers ways to manipulate and adapt it to anyone's needs.

By default, gocrawl uses the default net/http.Client to fetch the pages. This default will automatically follow redirects up to 10 times (see the [net/http doc for Client struct][netclient]). It is possible to provide a custom Fetcher interface implementation using the Crawler Options.

## Installation and dependencies

gocrawl depends on the following userland libraries:

*    [goquery][]
*    [purell][]
*    [robotstxt.go][robots]

Because of its dependency on goquery, **it requires Go's experimental html package to be installed [by following these instructions][exp] prior to the installation of gocrawl**.

Once this is done, gocrawl may be installed as usual:

`go get github.com/PuerkitoBio/gocrawl`

## TODOs

*    Cleanup workers once idle for a given duration.
*    Reset internal fields on Crawler.Run(), may be called multiple times on same instance.
*    Manage robots.txt error scenarios (what to do?).
*    Standardize log output.
*    Doc, examples.

[goquery]: https://github.com/PuerkitoBio/goquery
[robots]: https://github.com/temoto/robotstxt.go
[netclient]: http://golang.org/pkg/net/http/#Client
[purell]: https://github.com/PuerkitoBio/purell
[exp]: http://code.google.com/p/go-wiki/wiki/InstallingExp
