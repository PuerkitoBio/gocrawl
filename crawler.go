// Package gocrawl is a polite, slim and concurrent web crawler written in Go.
package gocrawl

import (
	"reflect"
	"strings"
	"sync"
)

// Communication from worker to the master crawler, about the crawling of a URL
type workerResponse struct {
	ctx           *URLContext
	visited       bool
	harvestedURLs interface{}
	host          string
	idleDeath     bool
}

// Crawler is the web crawler that processes URLs and manages the workers.
type Crawler struct {
	// Options configures the Crawler, refer to the Options type for documentation.
	Options *Options

	// Internal fields
	logFunc         func(LogFlags, string, ...interface{})
	push            chan *workerResponse
	enqueue         chan interface{}
	stop            chan struct{}
	wg              *sync.WaitGroup
	pushPopRefCount int
	visits          int

	// keep lookups in maps, O(1) access time vs O(n) for slice. The empty struct value
	// is of no use, but this is the smallest type possible - it uses no memory at all.
	visited map[string]struct{}
	hosts   map[string]struct{}
	workers map[string]*worker
}

// NewCrawlerWithOptions returns a Crawler initialized with the
// provided Options.
func NewCrawlerWithOptions(opts *Options) *Crawler {
	ret := new(Crawler)
	ret.Options = opts
	return ret
}

// NewCrawler returns a Crawler initialized with the default Options'
// values and the provided Extender. It is highly recommended to set
// at least the Options.RobotUserAgent to the custom name of your crawler
// before using the returned Crawler. Refer to the Options type documentation
// for details.
func NewCrawler(ext Extender) *Crawler {
	return NewCrawlerWithOptions(NewOptions(ext))
}

// Run starts the crawling process, based on the given seeds and the current
// Options settings. Execution stops either when MaxVisits is reached (if specified)
// or when no more URLs need visiting. If an error occurs, it is returned (if
// MaxVisits is reached, the error ErrMaxVisits is returned).
func (c *Crawler) Run(seeds interface{}) error {
	// Helper log function, takes care of filtering based on level
	c.logFunc = getLogFunc(c.Options.Extender, c.Options.LogFlags, -1)

	seeds = c.Options.Extender.Start(seeds)
	ctxs := c.toURLContexts(seeds, nil)
	c.init(ctxs)

	// Start with the seeds, and loop till death
	c.enqueueUrls(ctxs)
	err := c.collectUrls()

	c.Options.Extender.End(err)
	return err
}

// Initialize the Crawler's internal fields before a crawling execution.
func (c *Crawler) init(ctxs []*URLContext) {
	// Initialize the internal hosts map
	c.hosts = make(map[string]struct{}, len(ctxs))
	for _, ctx := range ctxs {
		// Add this normalized URL's host if it is not already there.
		if _, ok := c.hosts[ctx.normalizedURL.Host]; !ok {
			c.hosts[ctx.normalizedURL.Host] = struct{}{}
		}
	}

	hostCount := len(c.hosts)
	l := len(ctxs)
	c.logFunc(LogTrace, "init() - seeds length: %d", l)
	c.logFunc(LogTrace, "init() - host count: %d", hostCount)
	c.logFunc(LogInfo, "robot user-agent: %s", c.Options.RobotUserAgent)

	// Create a shiny new WaitGroup
	c.wg = new(sync.WaitGroup)

	// Initialize the visits fields
	c.visited = make(map[string]struct{}, l)
	c.pushPopRefCount, c.visits = 0, 0

	// Create the workers map and the push channel (the channel used by workers
	// to communicate back to the crawler)
	c.stop = make(chan struct{})
	if c.Options.SameHostOnly {
		c.workers, c.push = make(map[string]*worker, hostCount),
			make(chan *workerResponse, hostCount)
	} else {
		c.workers, c.push = make(map[string]*worker, c.Options.HostBufferFactor*hostCount),
			make(chan *workerResponse, c.Options.HostBufferFactor*hostCount)
	}
	// Create and pass the enqueue channel
	c.enqueue = make(chan interface{}, c.Options.EnqueueChanBuffer)
	c.setExtenderEnqueueChan()
}

// Set the Enqueue channel on the extender, based on the naming convention.
func (c *Crawler) setExtenderEnqueueChan() {
	defer func() {
		if err := recover(); err != nil {
			// Panic can happen if the field exists on a pointer struct, but that
			// pointer is nil.
			c.logFunc(LogError, "cannot set the enqueue channel: %s", err)
		}
	}()

	// Using reflection, check if the extender has a `EnqueueChan` field
	// of type `chan<- interface{}`. If it does, set it to the crawler's
	// enqueue channel.
	v := reflect.ValueOf(c.Options.Extender)
	el := v.Elem()
	if el.Kind() != reflect.Struct {
		c.logFunc(LogInfo, "extender is not a struct, cannot set the enqueue channel")
		return
	}
	ec := el.FieldByName("EnqueueChan")
	if !ec.IsValid() {
		c.logFunc(LogInfo, "extender.EnqueueChan does not exist, cannot set the enqueue channel")
		return
	}
	t := ec.Type()
	if t.Kind() != reflect.Chan || t.ChanDir() != reflect.SendDir {
		c.logFunc(LogInfo, "extender.EnqueueChan is not of type chan<-interface{}, cannot set the enqueue channel")
		return
	}
	tt := t.Elem()
	if tt.Kind() != reflect.Interface || tt.NumMethod() != 0 {
		c.logFunc(LogInfo, "extender.EnqueueChan is not of type chan<-interface{}, cannot set the enqueue channel")
		return
	}
	src := reflect.ValueOf(c.enqueue)
	ec.Set(src)
}

// Launch a new worker goroutine for a given host.
func (c *Crawler) launchWorker(ctx *URLContext) *worker {
	// Initialize index and channels
	i := len(c.workers) + 1
	pop := newPopChannel()

	// Create the worker
	w := &worker{
		host:    ctx.normalizedURL.Host,
		index:   i,
		push:    c.push,
		pop:     pop,
		stop:    c.stop,
		enqueue: c.enqueue,
		wg:      c.wg,
		logFunc: getLogFunc(c.Options.Extender, c.Options.LogFlags, i),
		opts:    c.Options,
	}

	// Increment wait group count
	c.wg.Add(1)

	// Launch worker
	go w.run()
	c.logFunc(LogInfo, "worker %d launched for host %s", i, w.host)
	c.workers[w.host] = w

	return w
}

// Check if the specified URL is from the same host as its source URL, or if
// nil, from the same host as one of the seed URLs.
func (c *Crawler) isSameHost(ctx *URLContext) bool {
	// If there is a source URL, then just check if the new URL is from the same host
	if ctx.normalizedSourceURL != nil {
		return ctx.normalizedURL.Host == ctx.normalizedSourceURL.Host
	}

	// Otherwise, check if the URL is from one of the seed hosts
	_, ok := c.hosts[ctx.normalizedURL.Host]
	return ok
}

// Enqueue the URLs returned from the worker, as long as it complies with the
// selection policies.
func (c *Crawler) enqueueUrls(ctxs []*URLContext) (cnt int) {
	for _, ctx := range ctxs {
		var isVisited, enqueue bool

		// Cannot directly enqueue a robots.txt URL, since it is managed as a special case
		// in the worker (doesn't return a response to crawler).
		if ctx.IsRobotsURL() {
			continue
		}
		// Check if it has been visited before, using the normalized URL
		_, isVisited = c.visited[ctx.normalizedURL.String()]

		// Filter the URL
		if enqueue = c.Options.Extender.Filter(ctx, isVisited); !enqueue {
			// Filter said NOT to use this url, so continue with next
			c.logFunc(LogIgnored, "ignore on filter policy: %s", ctx.normalizedURL)
			continue
		}

		// Even if filter said to use the URL, it still MUST be absolute, http(s)-prefixed,
		// and comply with the same host policy if requested.
		if !ctx.normalizedURL.IsAbs() {
			// Only absolute URLs are processed, so ignore
			c.logFunc(LogIgnored, "ignore on absolute policy: %s", ctx.normalizedURL)

		} else if !strings.HasPrefix(ctx.normalizedURL.Scheme, "http") {
			c.logFunc(LogIgnored, "ignore on scheme policy: %s", ctx.normalizedURL)

		} else if c.Options.SameHostOnly && !c.isSameHost(ctx) {
			// Only allow URLs coming from the same host
			c.logFunc(LogIgnored, "ignore on same host policy: %s", ctx.normalizedURL)

		} else {
			// All is good, visit this URL (robots.txt verification is done by worker)

			// Possible caveat: if the normalization changes the host, it is possible
			// that the robots.txt fetched for this host would differ from the one for
			// the unnormalized host. However, this should be rare, and is a weird
			// behaviour from the host (i.e. why would site.com differ in its rules
			// from www.site.com) and can be fixed by using a different normalization
			// flag. So this is an acceptable behaviour for gocrawl.

			// Launch worker if required, based on the host of the normalized URL
			w, ok := c.workers[ctx.normalizedURL.Host]
			if !ok {
				// No worker exists for this host, launch a new one
				w = c.launchWorker(ctx)
				// Automatically enqueue the robots.txt URL as first in line
				if robCtx, e := ctx.getRobotsURLCtx(); e != nil {
					c.Options.Extender.Error(newCrawlError(ctx, e, CekParseRobots))
					c.logFunc(LogError, "ERROR parsing robots.txt from %s: %s", ctx.normalizedURL, e)
				} else {
					c.logFunc(LogEnqueued, "enqueue: %s", robCtx.url)
					c.Options.Extender.Enqueued(robCtx)
					w.pop.stack(robCtx)
				}
			}

			cnt++
			c.logFunc(LogEnqueued, "enqueue: %s", ctx.url)
			c.Options.Extender.Enqueued(ctx)
			w.pop.stack(ctx)
			c.pushPopRefCount++

			// Once it is stacked, it WILL be visited eventually, so add it to the visited slice
			// (unless denied by robots.txt, but this is out of our hands, for all we
			// care, it is visited).
			if !isVisited {
				// The visited map works with the normalized URL
				c.visited[ctx.normalizedURL.String()] = struct{}{}
			}
		}
	}
	return
}

// This is the main loop of the crawler, waiting for responses from the workers
// and processing these responses.
func (c *Crawler) collectUrls() error {
	defer func() {
		c.logFunc(LogInfo, "waiting for goroutines to complete...")
		c.wg.Wait()
		c.logFunc(LogInfo, "crawler done.")
	}()

	for {
		// By checking this after each channel reception, there is a bug if the worker
		// wants to reenqueue following an error or a redirection. The pushPopRefCount
		// temporarily gets to zero before the new URL is enqueued. Check the length
		// of the enqueue channel to see if this is really over, or just this temporary
		// state.
		//
		// Check if refcount is zero - MUST be before the select statement, so that if
		// no valid seeds are enqueued, the crawler stops.
		if c.pushPopRefCount == 0 && len(c.enqueue) == 0 {
			c.logFunc(LogInfo, "sending STOP signals...")
			close(c.stop)
			return nil
		}

		select {
		case res := <-c.push:
			// Received a response, check if it contains URLs to enqueue
			if res.visited {
				c.visits++
				if c.Options.MaxVisits > 0 && c.visits >= c.Options.MaxVisits {
					// Limit reached, request workers to stop
					c.logFunc(LogInfo, "sending STOP signals...")
					close(c.stop)
					return ErrMaxVisits
				}
			}
			if res.idleDeath {
				// The worker timed out from its Idle TTL delay, remove from active workers
				delete(c.workers, res.host)
				c.logFunc(LogInfo, "worker for host %s cleared on idle policy", res.host)
			} else {
				c.enqueueUrls(c.toURLContexts(res.harvestedURLs, res.ctx.url))
				c.pushPopRefCount--
			}

		case enq := <-c.enqueue:
			// Received a command to enqueue a URL, proceed
			ctxs := c.toURLContexts(enq, nil)
			c.logFunc(LogTrace, "receive url(s) to enqueue %v", ctxs)
			c.enqueueUrls(ctxs)
		case <-c.stop:
			return ErrInterrupted
		}
	}
}

// Stop terminates the crawler.
func (c *Crawler) Stop() {
	defer func() {
		if err := recover(); err != nil {
			c.logFunc(LogError, "error when manually stopping crawler: %s", err)
		}
	}()

	// this channel may be closed already
	close(c.stop)
}
