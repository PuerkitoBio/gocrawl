# gocrawl

gocrawl is a polite, slim and concurrent web crawler written in Go.

## Features

*    Full control over the URLs to visit, inspect and query (using a pre-initialized [goquery][] document)
*    Crawl delays applied per host
*    Obedience to robots.txt rules (using [robotstxt.go][robots] go library)
*    Concurrent execution using goroutines
*    Configurable logging
*    Open, customizable design by providing hooks into the execution logic

## Installation and dependencies

gocrawl depends on the following userland libraries:

*    [goquery][]
*    [purell][]
*    [robotstxt.go][robots]

Because of its dependency on goquery, **it requires Go's experimental html package to be installed [by following these instructions][exp] prior to the installation of gocrawl (replace the second step with `hg clone -r d9ff34d481bc https://code.google.com/p/go go-exp`, since this is the last revision that compiles with Go1.0.3)**.

Once this is done, gocrawl may be installed as usual:

`go get github.com/PuerkitoBio/gocrawl`

## Changelog

*    **v0.3,2** : Fix the high CPU usage when waiting for a crawl delay.
*    **v0.3.1** : Export the `HttpClient` variable used by the default `Fetch()` implementation (see [issue #9][i9]).
*    **v0.3.0** : **BEHAVIOR CHANGE** filter done with normalized URL, fetch done with original, non-normalized URL (see [issue #10][i10]).
*    **v0.2.0** : **BREAKING CHANGES** rework extension/hooks.
*    **v0.1.0** : Initial release.

## Example

From `example_test.go`:

```Go
package gocrawl

import (
  "github.com/PuerkitoBio/goquery"
  "net/http"
  "net/url"
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
func (this *ExampleExtender) Filter(u *url.URL, src *url.URL, isVisited bool, origin EnqueueOrigin) (bool, int, HeadRequestMode) {
  // Priority (2nd return value) is ignored at the moment
  return !isVisited && rxOk.MatchString(u.String()), 0, HrmDefault
}

func ExampleCrawl() {
  // Set custom options
  opts := NewOptions(new(ExampleExtender))
  opts.CrawlDelay = 1 * time.Second
  opts.LogFlags = LogAll

  // Play nice with ddgo when running the test!
  opts.MaxVisits = 2

  // Create crawler and start at root of duckduckgo
  c := NewCrawlerWithOptions(opts)
  c.Run("https://duckduckgo.com/")

  // Remove "x" before Output: to activate the example (will run on go test)

  // xOutput: voluntarily fail to see log output
}
```

## API

Gocrawl can be described as a minimalist web crawler (hence the "slim" tag, at <1000 sloc), providing the basic engine upon which to build a full-fledged indexing machine with caching, persistence and staleness detection logic, or to use as is for quick and easy crawling. Gocrawl itself does not attempt to detect staleness of a page, nor does it implement a caching mechanism. If an URL is enqueued to be processed, it *will* make a request to fetch it (provided it is allowed by robots.txt - hence the "polite" tag). And there is no prioritization among the URLs to process, it assumes that all enqueued URLs must be visited at some point, and that the order in which they are is unimportant - *subject to change in the future, prioritization __may__ get implemented*.

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

The one and only public function is `Run(seeds ...string) EndReason` which take a variadic string argument, the base URLs used to start crawling. It ends when there are no more URLs waiting to be visited, or when the `Options.MaxVisit` number is reached.

### Options

The Options type is detailed in the next section, and it offers a single constructor, `NewOptions(Extender)`, which returns an initialized options object with defaults and the specified `Extender` implementation.

### Hooks and customizations
<a name="hc" />

The `Options` type provides the hooks and customizations offered by gocrawl. All but `Extender` are optional and have working defaults.

*    **UserAgent** : The user-agent string used to fetch the pages. Defaults to the Firefox 15 on Windows user-agent string.

*    **RobotUserAgent** : The robot's user-agent string used to fetch and query robots.txt for permission to crawl an URL. Defaults to `Googlebot (gocrawl vM.m)` where `M.m` is the major and minor version of gocrawl. See the [robots exclusion protocol][robprot] ([full specification as interpreted by Google here][robspec]) for details about the rule-matching based on the robot's user agent. It is good practice to include contact information in the user agent should the site owner need to contact you.

*    **MaxVisits** : The maximum number of pages *visited* before stopping the crawl. Probably more useful for development purposes. Note that the Crawler will send its stop signal once this number of visits is reached, but workers may be in the process of visiting other pages, so when the crawling stops, the number of pages visited will be *at least* MaxVisits, possibly more (worst case is `MaxVisits + number of active workers`). Defaults to zero, no maximum.

*    **EnqueueChanBuffer** : The size of the buffer for the Enqueue channel (the channel that allows the extender to arbitrarily enqueue new URLs in the crawler). Defaults to 100.

*    **HostBufferFactor** : The factor (multiplier) for the size of the workers map and the communication channel when SameHostOnly is set to `false`. When SameHostOnly is `true`, the Crawler knows exactly the required size (the number of different hosts based on the seed URLs), but when it is `false`, the size may grow exponentially. By default, a factor of 10 is used (size is set to 10 times the number of different hosts based on the seed URLs).

*    **CrawlDelay** : The time to wait between each request to the same host. The delay starts as soon as the response is received from the host. This is a `time.Duration` type, so it can be specified with `5 * time.Second` for example (which is the default value, 5 seconds). **If a crawl delay is specified in the robots.txt file, in the group matching the robot's user-agent, by default this delay is used instead**. Crawl delay can be customized further by implementing the `ComputeDelay` extender function.

*    **WorkerIdleTTL** : The idle time-to-live allowed for a worker before it is cleared (its goroutine terminated). Defaults to 10 seconds. The crawl delay is not part of idle time, this is specifically the time when the worker is available, but there are no URLs to process.

*    **SameHostOnly** : Limit the URLs to enqueue only to those links targeting the same host, which is `true` by default.

*    **HeadBeforeGet** : Asks the crawler to issue a HEAD request (and a subsequent `RequestGet()` extender method call) before making the eventual GET request. This is set to `false` by default. See also the `Filter()` extender method.

*    **URLNormalizationFlags** : The flags to apply when normalizing the URL using the [purell][] library. The URLs found by crawling a page are normalized before being submitted to the `Filter` extender function (to determine if they should be visited or not). Defaults to the most aggressive normalization allowed by purell, `purell.FlagsAllGreedy`.

*    **LogFlags** : The level of verbosity for logging. Defaults to errors only (`LogError`). Can be a set of flags (i.e. `LogError | LogTrace`).

*    **Extender** : The instance implementing the `Extender` interface. This implements the various callbacks offered by gocrawl. Must be specified when creating a `Crawler` (or when creating an `Options` to pass to `NewCrawlerWithOptions` constructor). A default extender is provided as a valid default implementation, `DefaultExtender`. It can be used by [embedding it as an anonymous field][gotalk] to implement a custom extender when not all methods need customization (see the example above).

### The Extender interface

This last option field, `Extender`, is crucial in using gocrawl, so here are the details for each callback function required by the `Extender` interface.

*    **Start** : `Start(seeds []string) []string`. Called when `Run` is called on the crawler, with the seeds passed to `Run` as argument. It returns a slice of strings that will be used as actual seeds, so that this callback can control which seeds are passed to the crawler.

*    **End** : `End(reason EndReason)`. Called when the crawling ends, with an [`EndReason`][er] flag as argument. This same `EndReason` flag is also returned from the `Crawler.Run()` function.

*    **Error** : `Error(err *CrawlError)`. Called when an error occurs. Errors do **not** stop the crawling execution. A [`CrawlError`][ce] instance is passed as argument. This specialized error implementation includes - among other interesting fields - a `Kind` field that indicates the step where the error occurred.

*    **Log** : `Log(logFlags LogFlags, msgLevel LogFlags, msg string)`. The logging function. By default, prints to the standard error (Stderr), and outputs only the messages with a level included in the `LogFlags` option. If a custom `Log()` method is implemented, it is up to you to validate if the message should be considered, based on the level of verbosity requested (i.e. `if logFlags&msgLevel == msgLevel ...`), since the method always gets called for all messages.

*    **ComputeDelay** : `ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration`. Called by a worker before requesting a URL. Arguments are the host's name, the crawl delay information (delays from the Options struct, from the robots.txt, and the last used delay), and the last fetch information, so that it is possible to adapt to the current responsiveness of the host. It returns the delay to use. 

*    **Fetch** : `Fetch(u *url.URL, userAgent string, headRequest bool) (*http.Response, error)`. Called by a worker to request the URL. The `DefaultExtender.Fetch()` implementation uses the public `HttpClient` variable (a custom `http.Client`) to fetch the pages *without* following redirections, instead returning an error so that the worker can enqueue the redirect-to URL. This enforces the whitelisting by the `Filter()` of every URL fetched by the crawling process. If `headRequest` is `true`, a HEAD request is made instead of a GET. Note that as of gocrawl v0.3, `Fetch` is called with the non-normalized URL.

    Internally, gocrawl sets its http.Client's `CheckRedirect()` function field to a custom implementation that follows redirections for robots.txt URLs only (since a redirect on robots.txt still means that the site owner wants us to use these rules for this host). The worker is aware of the `*gocrawl.EnqueueRedirectError` error, so if a non-robots.txt URL asks for a redirection, `CheckRedirect()` returns an instance of this error, and the worker recognizes this and enqueues the redirect-to URL, stopping the processing of the current URL. It is possible to provide a custom `Fetch()` implementation based on the same logic. Any `CheckRedirect()` implementation that returns a `*gocrawl.EnqueueRedirectError` error will behave this way - that is, the worker will detect this error and will enqueue the redirect-to URL. See the source files ext.go and worker.go for details.

    The `HttpClient` variable being public, it is possible to customize it so that it uses another `CheckRedirect()` function, or a different `Transport` object, etc. This customization should be done prior to starting the crawler. It will then be used by the default `Fetch()` implementation, or it can also be used by a custom `Fetch()` if required. Note that this client is shared by all crawlers in your application. Should you need different http clients per crawler in the same application, a custom `Fetch()` using a private `http.Client` instance should be provided.

*    **RequestGet** : `RequestGet(headRes *http.Response) bool`. Indicates if the crawler should proceed with a GET request based on the HEAD request's response. This method is only called if a HEAD was requested (either via the global `HeadBeforeGet` option, or via the `Filter()` return value). The default implementation returns `true` if the HEAD response status code was 2xx.

*    **RequestRobots** : `RequestRobots(u *url.URL, robotAgent string) (request bool, data []byte)`. Asks whether the robots.txt URL should be fetched. If `false` is returned as first value, the `data` value is considered to be the robots.txt cached content, and is used as such (if it is empty, it behaves as if there was no robots.txt). The `DefaultExtender.RequestRobots` implementation returns `true, nil`.

*    **FetchedRobots** : `FetchedRobots(res *http.Response)`. Called when the robots.txt URL has been fetched from the host, so that it is possible to cache its content and feed it back to future `RequestRobots()` calls. By default, this is a no-op.

*    **Filter** : `Filter(u *url.URL, from *url.URL, isVisited bool, origin EnqueueOrigin) (enqueue bool, priority int, headRequest HeadRequestMode)`. Called when deciding if a URL should be enqueued for visiting. It receives the target `*url.URL` link in normalized form, the source `*url.URL` link (where this URL was found, `nil` for the seeds or for manually enqueued URLs, via the enqueue channel - see below), a `bool` is visited flag, indicating if this URL has already been visited in this crawling execution, and an origin (which can be extended to create custom origins, starting at `EoCustomStart` instead of `iota`). It returns a `bool` flag ordering gocrawl to visit (`true`) or ignore (`false`) the URL, a priority value, **ignored at the moment**, and whether or not to request a HEAD before a GET (if `HrmDefault` is returned, the global `HeadBeforeGet` setting is used, while `HrmRequest` and `HrmIgnore` override the global setting). Even if the function returns `true` to enqueue the URL for visiting, the normalized form of the URL must still comply to these rules:

1. It must be an absolute URL 
2. It must have a `http/https` scheme
3. It must have the same host if the `SameHostOnly` flag is set

    The `DefaultExtender.Filter` implementation returns `true` if the URL has not been visited yet (the *visited* flag is based on the normalized version of the URLs), false otherwise. It returns `0` for the priority and `HrmDefault` for the head request.

*    **Enqueued** : `Enqueued(u *url.URL, from *url.URL)`. Called when a URL has been enqueued by the crawler. An enqueued URL may still be disallowed by a robots.txt policy, so it may end up *not* being fetched.

*    **Visit** : `Visit(res *http.Response, doc *goquery.Document) (harvested []*url.URL, findLinks bool)`. Called when visiting a URL. It receives a `*http.Response` response object, along with a ready-to-use `*goquery.Document` object (or `nil` if the response body could not be parsed). It returns a slice of `*url.URL` links to process, and a `bool` flag indicating if gocrawl should find the links himself. When this flag is `true`, the slice of URLs is ignored and gocrawl searches the goquery document for links to enqueue. When `false`, the returned slice of URLs is enqueued, if any. The `DefaultExtender.Visit` implementation returns `nil, true` so that links from a visited page are found and processed.

*    **Visited** : `Visited(u *url.URL, harvested []*url.URL)`. Called after a page has been visited. The URLs found during the visit (either by the `Visit` function or by gocrawl) are passed as argument.

*    **Disallowed** : `Disallowed(u *url.URL)`. Called when an enqueued URL gets denied acces by a robots.txt policy.

Finally, by convention, if a field named `EnqueueChan` with the very specific type of `chan<- *CrawlerCommand` exists and is accessible on the Extender instance, this field will get set to the enqueue channel, which accepts a pointer to a `CrawlerCommand` structure, specifying a URL and an origin. This URL will then be processed by the crawler as if it had been harvested from a visit. It will trigger a call to `Filter()` and, if allowed, will get fetched and visited.

The `DefaultExtender` structure has a valid `EnqueueChan` field, so if it is embedded as an anonymous field in a custom Extender structure, this structure automatically gets the `EnqueueChan` functionality.

This channel can be useful to arbitrarily enqueue URLs that would otherwise not be processed by the crawling process. For example, if an URL raises a server error (status code 5xx), it could be re-enqueued in the `Error()` extender function, so that another fetch is attempted.

## Thanks

* Richard Penman
* Dmitry Bondarenko
* Markus Sonderegger

## License

The [BSD 3-Clause license][bsd].

[bsd]: http://opensource.org/licenses/BSD-3-Clause
[goquery]: https://github.com/PuerkitoBio/goquery
[robots]: https://github.com/temoto/robotstxt.go
[purell]: https://github.com/PuerkitoBio/purell
[exp]: http://code.google.com/p/go-wiki/wiki/InstallingExp
[robprot]: http://www.robotstxt.org/robotstxt.html
[robspec]: https://developers.google.com/webmasters/control-crawl-index/docs/robots_txt
[godoc]: http://go.pkgdoc.org/github.com/PuerkitoBio/gocrawl
[er]: http://go.pkgdoc.org/github.com/PuerkitoBio/gocrawl#EndReason
[ce]: http://go.pkgdoc.org/github.com/PuerkitoBio/gocrawl#CrawlError
[gotalk]: http://talks.golang.org/2012/chat.slide#32
[i10]: https://github.com/PuerkitoBio/gocrawl/issues/10
[i9]: https://github.com/PuerkitoBio/gocrawl/issues/9
