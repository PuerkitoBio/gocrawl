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

## Changelog

*    **v0.2.0** : *In development* **BREAKING CHANGES** rework extension/hooks.
*    **v0.1.0** : Initial release.

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

// Create the Extender implementation, based on the gocrawl-provided DefaultExtender,
// because we don't want/need to override all methods.
type ExampleExtender struct {
  DefaultExtender // Will use the default implementation of all but Visit() and Filter()
}

// Override Visit for our need.
func (this *ExampleExtender) Visit(res *http.Response, doc *goquery.Document) ([]*url.URL, bool) {
  // Use the goquery document or res.Body to manipulate the data
  // ...

  // Return nil and true - let gocrawl find the links
  return nil, true
}

// Override Filter for our need.
func (this *ExampleExtender) Filter(u *url.URL, src *url.URL, isVisited bool) (bool, int) {
  // Priority (2nd return value) is ignored at the moment
  return rxOk.MatchString(u.String()), 0
}

func ExampleCrawl() {
  // Set custom options
  opts := NewOptions(new(ExampleExtender))
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
  // worker 1 - using crawl-delay: 1s
  // worker 1 - popped: http://duckduckgo.com
  // worker 1 - using crawl-delay: 1s
  // worker 1 - waiting for pop...
  // worker 1 - popped: http://duckduckgo.com/about.html
  // worker 1 - using crawl-delay: 1s
  // sending STOP signals...
  // waiting for goroutines to complete...
  // worker 1 - stop signal received.
  // worker 1 - worker done.
  // crawler done.
}
```

## API

gocrawl can be described as a minimalist web crawler (hence the "slim" tag, at ~700 sloc), providing the basic engine upon which to build a full-fledged indexing machine with caching, persistence and staleness detection logic, or to use as is for quick and easy crawling. For gocrawl itself does not attempt to detect staleness of a page, nor does it implement a caching mechanism. If an URL is enqueued to be processed, it *will* make a request to fetch it (provided it is allowed by robots.txt - hence the "polite" tag). And there is no prioritization among the URLs to process, it assumes that all enqueued URLs must be visited at some point, and that the order in which they are is unimportant (*subject to change in the future - prioritization __may__ get implemented*).

However, it does provide plenty of [hooks and customizations](#hc). Instead of trying to do everything and impose a way to do it, it offers ways to manipulate and adapt it to anyone's needs.

As usual, the complete godoc reference can be found [here][godoc].

### Design rationale

The major use-case behind gocrawl is to crawl some web pages while respecting the constraints of `robots.txt` policies and while applying a *good web citizen* crawl delay between each request to a given host. Hence the following design decisions:

* **Each host spawns its own worker (goroutine)** : This makes sense since it must first read its robots.txt data, and only then proceed sequentially, one request at a time, with the specified delay between each fetch. There are no constraints inter-host, so each separate worker can crawl independently.
* **The visitor function is called on the worker goroutine** : Again, this is ok because the crawl delay is likely bigger than the time required to parse the document, so this processing will usually *not* penalize the performance.
* **Edge cases with no crawl delay are supported, but not optimized** : In the rare but possible event when a crawl with no delay is needed (e.g.: on your own servers, or with permission outside busy hours, etc.), gocrawl accepts a null (zero) delay, but doesn't provide optimizations. That is, there is no "special path" in the code where visitor functions are de-coupled from the worker, or where multiple workers can be launched concurrently on the same host. (In fact, if this case is your *only* use-case, I would recommend *not* to use a library at all - since there is little value in it -, and simply use Go's standard libs and fetch at will with as many goroutines as are necessary.)

Although it could probably be used to crawl a massive amount of web pages (after all, this is *fetch, visit, enqueue, repeat ad nauseam*!), most realistic (and um... tested!) use-cases should be based on a well-known, well-defined limited bucket of seeds. Distributed crawling is your friend, should you need to move past this reasonable use.

### Crawler

The Crawler type controls the whole execution. It spawns worker goroutines and manages the URL queue. There are two helper constructors:

*    **NewCrawler(Extender)** : Creates a crawler with the specified `Extender` instance.
*    **NewCrawlerWithOptions(Options)** : Creates a crawler with a pre-initialized `*Options` instance.

The one and only public function is `Run(seeds ...string)` which take a variadic string argument, the base URLs used to start crawling. It ends when there are no more URLs waiting to be visited, or when the `Options.MaxVisit` number is reached.

### Options

The Options type is detailed in the next section, and it offers a single constructor, `NewOptions(Extender)`, which returns an initialized options object with defaults and the specified `Extender` implementation.

### Hooks and customizations
<a name="hc" />

The `Options` type provides the hooks and customizations offered by gocrawl. All but `Extender` are optional and have working defaults.

*    **UserAgent** : The user-agent string used to fetch the pages. Defaults to the Firefox 15 on Windows user-agent string.

*    **RobotUserAgent** : The robot's user-agent string used to fetch and query robots.txt for permission to crawl an URL. Defaults to `Googlebot (gocrawl vM.m)` where `M.m` is the major and minor version of gocrawl. Because of the way the [robots exclusion protocol][robprot] works ([full specification as interpreted by Google here][robspec]), the user-agent string should *start* with a well-known robot name so that it (most likely) behaves as intended by the site owner. It is good practice to include contact information should the site owner need to contact you.

*    **MaxVisits** : The maximum number of pages *visited* before stopping the crawl. Probably more useful for development purposes. Note that the Crawler will send its stop signal once this number of visits is reached, but workers may be in the process of visiting other pages, so when the crawling stops, the number of pages visited will be *at least* MaxVisits, possibly more (worst case is `MaxVisits + number of active workers`). Defaults to zero, no maximum.

*    **CrawlDelay** : The time to wait between each request to the same host. The delay starts as soon as the response is received from the host. This is a `time.Duration` type, so it can be specified with `5 * time.Second` for example (which is the default value, 5 seconds). **If a crawl delay is specified in the robots.txt file, in the group matching the robot's user-agent, by default this delay is used instead**. Crawl delay can be customized by implementing the `ComputeDelay` extender function.

*    **WorkerIdleTTL** : The idle time-to-live allowed for a worker before it is cleared (its goroutine terminated). Defaults to 10 seconds. The crawl delay is not part of idle time, this is specifically the time when the worker is available, but there are no URLs to process.

*    **SameHostOnly** : Limit the URLs to enqueue only to those links targeting the same host, which is `true` by default.

*    **URLNormalizationFlags** : The flags to apply when normalizing the URL using the [purell][] library. The URLs found by crawling a page are normalized before being submitted to the `Filter` extender function (to determine if they should be visited or not). Defaults to the most aggressive normalization allowed by purell, `purell.FlagsAllGreedy`.

*    **Logger** : An instance of Go's built-in `*log.Logger` type. It can be created by calling `log.New()`. By default, a logger that prints to the standard output is used.

*    **LogFlags** : The level of verbosity for the logger. Defaults to errors only (`LogError`). Can be a set of flags (i.e. `LogError | LogTrace`).

*    **Extender** : The instance implementing the `Extender` interface. This implements the various callbacks offered by gocrawl. Must be specified when creating a `Crawler` (or when creating an `Options` to pass to `NewCrawlerWithOptions` constructor). A default extender is provided as a valid default implementation, `DefaultExtender`. It can be used to implement a custom extender when not all methods need customization (see the example above).

This last option field, `Extender`, is crucial in using gocrawl, so here are the details for each callback function required by the `Extender` interface:

*    **Start** : `Start(seeds []string) []string`. Called when `Run` is called on the crawler, with the seeds passed to `Run` as argument. It returns a slice of strings that will be used as actual seeds, so that this callback can control which seeds are passed to the crawler.

*    **End** : `End(reason EndReason)`. Called when the crawling ends, with an [`EndReason`][er] flag as argument.

*    **Error** : `Error(err *CrawlError)`. Called when an error occurs. Errors do **not** stop the crawling execution. A [`CrawlError`][ce] instance is passed as argument. This specialized error implementation includes - among other interesting fields - an `ErrorKind` field that details the step where the error occurred.

*    **ComputeDelay** : `ComputeDelay(host string, optsDelay time.Duration, robotsDelay time.Duration, lastFetch time.Duration) time.Duration`. Called by a worker before requesting a URL. Arguments are the host's name, the configured crawl delay (from the Options), the robots.txt crawl delay (if any), and the duration of the last fetch, so that it is possible to adapt to the current responsiveness of the host. It returns the delay to use.

*    **Fetch** : `Fetch(u *url.URL, userAgent string) (*http.Response, error)`. The `DefaultExtender.Fetch` implementation uses the default `http.Client` to fetch the pages. It will automatically follow redirects up to 10 times (see the [net/http doc for Client struct][netclient]).

*    **RequestRobots** : `RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte)`. Asks whether the robots.txt URL should be fetched. If `false` is returned as first value, the `data` value is considered to be the robots.txt cached content, and is used as such. The `DefaultExtender.RequestRobots` implementation returns `true, nil`.

*    **FetchedRobots** : `FetchedRobots(res *http.Response)`. Called when the robots.txt URL has been fetched from the host.

*    **Filter** : `Filter(u *url.URL, from *url.URL, isVisited bool) (enqueue bool, priority int)`. Called when deciding if a URL should be enqueued for visiting. It receives the target `*url.URL` link, the source `*url.URL` link (where this URL was found, `nil` for the seeds), and a `bool` is visited flag, indicating if this URL has already been visited in this crawling execution. It returns a `bool` flag ordering gocrawl to visit (`true`) or ignore (`false`) the URL, and a priority value, **ignored at the moment**. However, even if the function returns true, the URL must still comply to these rules:

1. It must be an absolute URL 
2. It must have a `http/https` scheme
3. It must have the same host if the `SameHostOnly` flag is set

    The `DefaultExtender.Filter` implementation returns `true` if the URL has not been visited yet, false otherwise.

*    **Enqueued** : `Enqueued(u *url.URL, from *url.URL)`. Called when a URL has been enqueued by the crawler. An enqueued URL may still be disallowed by a robots.txt policy, so it may end up *not* being fetched.

*    **Visit** : `Visit(*http.Response, *goquery.Document) (harvested []*url.URL, findLinks bool)`. Called when visiting a URL. It receives a `*http.Response` response object, along with a ready-to-use `*goquery.Document` object (or `nil` if the response body could not be parsed). It returns a slice of `*url.URL` links to process, and a `bool` flag indicating if gocrawl should find the links himself. When this flag is `true`, the slice of URLs is ignored and gocrawl searches the goquery document for links to enqueue. When `false`, the returned slice of URLs is enqueued, if any. The `DefaultExtender.Visit` implementation returns `nil, true` so that links from a visited page are found and processed.

*    **Visited** : `Visited(u *url.URL, harvested []*url.URL)`. Called after a page has been visited. The URLs found during the visit (either by the `Visit` function or by gocrawl) are passed as argument.

*    **Disallowed** : `Disallowed(u *url.URL)`. Called when an enqueued URL gets denied acces by a robots.txt policy.

## Thanks

Richard Penman

## License

The [BSD 3-Clause license][bsd].

[bsd]: http://opensource.org/licenses/BSD-3-Clause
[goquery]: https://github.com/PuerkitoBio/goquery
[robots]: https://github.com/temoto/robotstxt.go
[netclient]: http://golang.org/pkg/net/http/#Client
[purell]: https://github.com/PuerkitoBio/purell
[exp]: http://code.google.com/p/go-wiki/wiki/InstallingExp
[robprot]: http://www.robotstxt.org/robotstxt.html
[robspec]: https://developers.google.com/webmasters/control-crawl-index/docs/robots_txt
[godoc]: http://go.pkgdoc.org/github.com/puerkitobio/gocrawl
[er]: http://go.pkgdoc.org/github.com/puerkitobio/gocrawl#EndReason
[ce]: http://go.pkgdoc.org/github.com/puerkitobio/gocrawl#CrawlError