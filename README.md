# gocrawl

gocrawl is a polite, slim and concurrent web crawler written in Go.

## Features

*    Full control over the URLs to visit, inspect and query (using a pre-initialized [goquery][] document)
*    Crawl delays applied per host
*    Obedience to robots.txt rules (using [robotstxt.go][robots] go library)
*    Concurrent execution using goroutines
*    Configurable logging using the builtin Go logger
*    Open, customizable design by providing hooks into the execution logic

## Installation and dependencies

gocrawl depends on the following userland libraries:

*    [goquery][]
*    [purell][]
*    [robotstxt.go][robots]

Because of its dependency on goquery, **it requires Go's experimental html package to be installed [by following these instructions][exp] prior to the installation of gocrawl**.

Once this is done, gocrawl may be installed as usual:

`go get github.com/PuerkitoBio/gocrawl`

## Example

From `example_test.go`:

```Go
package gocrawl

import (
  "github.com/PuerkitoBio/goquery"
  "log"
  "net/http"
  "net/url"
  "os"
  "regexp"
  "time"
)

// Only enqueue the root and paths beginning with an "a"
var rxOk = regexp.MustCompile(`http://duckduckgo\.com(/a.*)?$`)

func visitor(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
  // Use the goquery document or res.Body to manipulate the data
  // ...

  // Return nil and true - let gocrawl find the links
  return nil, true
}

func urlSelector(u *url.URL, src *url.URL, isVisited bool) bool {
  return rxOk.MatchString(u.String())
}

func ExampleCrawl() {
  // Set custom options
  opts := NewOptions(visitor, urlSelector)
  opts.CrawlDelay = 1 * time.Second
  opts.LogFlags = LogInfo
  opts.Logger = log.New(os.Stdout, "", 0)

  // Play nice with ddgo when running the test!
  opts.MaxVisits = 2

  // Create crawler and start at root of duckduckgo
  c := NewCrawlerWithOptions(opts)
  c.Run("http://duckduckgo.com/")

  // Output: robot user-agent: Googlebot (gocrawl v0.1)
  // worker 1 launched for host duckduckgo.com
  // worker 1 - waiting for pop...
  // worker 1 - popped: http://duckduckgo.com/robots.txt
  // worker 1 - popped: http://duckduckgo.com
  // worker 1 - waiting for pop...
  // worker 1 - popped: http://duckduckgo.com/about.html
  // sending STOP signals...
  // waiting for goroutines to complete...
  // worker 1 - stop signal received.
  // worker 1 - worker done.
  // crawler done.
}
```

## API

gocrawl can be described as a minimalist web crawler (hence the "slim" tag, at ~500 sloc), providing the basic engine upon which to build a full-fledged indexing machine with caching, persistence and staleness detection logic, or to use as is for quick and easy crawling. For gocrawl itself does not attempt to detect staleness of a page, nor does it implement a caching mechanism. If an URL is enqueued to be processed, it *will* make a request to fetch it (provided it is allowed by robots.txt - hence the "polite" tag). And there is no prioritization among the URLs to process, it assumes that all enqueued URLs must be visited at some point, and that the order in which they are is unimportant.

However, it does provide plenty of [hooks and customizations](#hc). Instead of trying to do everything and impose a way to do it, it offers ways to manipulate and adapt it to anyone's needs.

As usual, the complete godoc reference can be found [here][godoc].

### Crawler

The Crawler type controls the whole execution. It spawns worker goroutines and manages the URL queue. There are two helper constructors:

*    **NewCrawler(visitor, urlSelector)** : Creates a crawler with the specified visitor function and URL selector function (which may be `nil`).
*    **NewCrawlerWithOptions(options)** : Creates a crawler with a pre-initialized `*Options` instance.

The one and only public function is `Run(seeds ...string)` which take a variadic string argument, the base URLs used to start crawling. It ends when there are no more URLs waiting to be visited, or when the `Options.MaxVisit` number is reached.

### Options

The Options type is detailed in the next section, and it offers a single constructor, `NewOptions(visitor, urlSelector)`, which returns an initialized options object with defaults and the specified visitor function and URL selector function (which may be `nil`).

### Hooks and customizations
<a name="hc" />

The `Options` type provides the hooks and customizations offered by gocrawl. All but the `URLVisitor` are optional and have working defaults.

*    **UserAgent** : The user-agent string used to fetch the pages. Defaults to the Firefox 15 on Windows user-agent string.

*    **RobotUserAgent** : The robot's user-agent string used to check for robots.txt permission to fetch an URL. Defaults to `Googlebot (gocrawl vM.m)` where `M.m` is the major and minor version of gocrawl. Because of the way the [robots exclusion protocol][robprot] works ([full specification as interpreted by Google here][robspec]), the user-agent string should *start* with a well-known robot name so that it (most likely) behaves as intended by the site owner.

*    **MaxVisits** : The maximum number of pages *visited* before stopping the crawl. Probably more useful for development purposes. Note that the Crawler will send its stop signal once this number of visits is reached, but workers may be in the process of visiting other pages, so when the crawling stops, the number of pages visited will be *at least* MaxVisits, possibly more (worst case is `MaxVisits + number of active workers`). Defaults to zero, no maximum.

*    **CrawlDelay** : The time to wait between each request to the same host. The delay starts as soon as the response is received from the host. This is a `time.Duration` type, so it can be specified with `5 * time.Second` for example (which is the default value, 5 seconds). **If a crawl delay is specified in the robots.txt file, in the group matching the robot's user-agent, this delay is used instead**.

*    **WorkerIdleTTL** : The idle time-to-live allowed for a worker before it is cleared (its goroutine terminated). Defaults to 10 seconds.

*    **SameHostOnly** : A quick and easy configuration to limit the selected URLs only to those links targeting the same host, which is `true` by default.

*    **URLNormalizationFlags** : The flags to apply when normalizing the URL using the [purell][] library. The URLs found by crawling a page are normalized before being submitted to the URL selection criteria (to determine if they should be visited or not). Defaults to the most aggressive normalization allowed by purell, `purell.FlagsAllGreedy`.

*    **URLVisitor** : The function to be called when visiting a URL. It receives a `*http.Response` response object, along with a ready-to-use `*goquery.Document` object (or `nil` if the response body could not be parsed). It returns a slice of `*url.URL` links to enqueue, and a `bool` flag indicating if gocrawl should find the links himself. When this flag is `true`, the slice of URLs is ignored and gocrawl searches the goquery document for links to enqueue. When `false`, the returned slice of URLs is enqueued, if any. Note that this function is called from a worker goroutine. If no visitor function is provided, the crawling process is mostly useless, no additional URLs will be enqueued (gocrawl will not process links found in the document), so *this option should be considered mandatory*.

*    **URLSelector** : The function to be called when deciding if a URL should be enqueued for visiting. It receives the target `*url.URL` link, the source `*url.URL` link (where this URL was found, `nil` for the seeds), and a `bool` is visited flag, indicating if this URL has already been visited in this crawling execution. It returns a `bool` flag ordering gocrawl to visit (`true`) or ignore (`false`) the URL. However, even if the function returns true, the URL must still comply to these rules:

1. It must be an absolute URL 
2. It must have a `http/https` scheme
3. It must have the same host if the `SameHostOnly` flag is set

    *The selector is optional*, if none is provided, the 3 rules above are applied, and the link is visited only once (if the is visited flag is true, it is not visited again). The selector function is called from the Crawler, which means it can *potentially* block all workers if it is too slow (see the `push` channel in the code - it is a buffered channel, but if the selector is *very* slow it could still be a problem). Make sure this function is fast if you care about performance (i.e. regular expressions pre-compiled and cached, DB caching outside the function, etc.).

*    **Fetcher** : The Fetcher interface defines only one method, `Fetch(u *url.URL, userAgent string) (*http.Response, error)`. If no custom fetcher is specified, a default fetcher based on the default `http.Client` is used to fetch the pages. It will automatically follow redirects up to 10 times (see the [net/http doc for Client struct][netclient]).

*    **Logger** : An instance of Go's built-in `*log.Logger` type. It can be created by calling `log.New()`. By default, a logger that prints to the standard output is used.

*    **LogFlags** : The level of verbosity for the logger. Defaults to errors only (`LogError`). Can be a set of flags (i.e. `LogError | LogTrace`).

[goquery]: https://github.com/PuerkitoBio/goquery
[robots]: https://github.com/temoto/robotstxt.go
[netclient]: http://golang.org/pkg/net/http/#Client
[purell]: https://github.com/PuerkitoBio/purell
[exp]: http://code.google.com/p/go-wiki/wiki/InstallingExp
[robprot]: http://www.robotstxt.org/robotstxt.html
[robspec]: https://developers.google.com/webmasters/control-crawl-index/docs/robots_txt
[godoc]: http://go.pkgdoc.org/github.com/puerkitobio/gocrawl
