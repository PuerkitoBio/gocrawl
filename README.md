# gocrawl

gocrawl is a polite, slim and concurrent web crawler.

**It is currently in a very early stage of active development, unfit for any serious use. It will kill kittens. Don't. Not yet.**

## Features

*    Configurable URLs to visit, inspect and query (using a pre-initialized [goquery][] document)
*    Crawl delays applied per host
*    Obedience to robots.txt rules (using [robotstxt][] go library)
*    Configurable concurrency
*    Loggable using the builtin configurable Go logger
*    ...and possibly more, we'll see!

gocrawl does not attempt to detect staleness of a page, nor does it implement a caching mechanism. If an URL is enqueued to be processed, it *will* make a request to fetch it. However, it provides some hooks where some URL analysis can take place to alter its behaviour (based on the last visited date of the URL, saved in a persistent store, for example). Instead of trying to do everything and impose a way to do it, it offers ways to manipulate and adapt it to anyone's needs (hopefully!).

Likewise, there is no prioritization among the URLs to process. It assumes that all enqueued URLs must be visited at some point, and that the order in which they are is unimportant.

By default, uses the default net/http.Client. This default will automatically follow redirects up to 10 times (see the [net/http doc for Client struct][netclient]). It will be possible to provide a custom Fetcher interface implementation.

## TODOs

*    Cleanup workers once idle for a given duration.
*    Reset internal fields on Crawler.Run(), may be called multiple times on same instance.
*    Manage robots.txt error scenarios (what to do?).
*    Standardize log output.
*    More tests, especially with fetcher for controlled, static data, to check some behaviour.

[goquery]: https://github.com/PuerkitoBio/goquery
[robotstxt]: https://github.com/temoto/robotstxt.go
[netclient]: http://golang.org/pkg/net/http/#Client
