package gocrawl

import (
	"time"

	"github.com/PuerkitoBio/purell"
)

// Default options
const (
	DefaultUserAgent          string                    = `Mozilla/5.0 (Windows NT 6.1; rv:15.0) gocrawl/0.4 Gecko/20120716 Firefox/15.0a2`
	DefaultRobotUserAgent     string                    = `Googlebot (gocrawl v0.4)`
	DefaultEnqueueChanBuffer  int                       = 100
	DefaultHostBufferFactor   int                       = 10
	DefaultCrawlDelay         time.Duration             = 5 * time.Second
	DefaultIdleTTL            time.Duration             = 10 * time.Second
	DefaultNormalizationFlags purell.NormalizationFlags = purell.FlagsAllGreedy
)

// Options contains the configuration for a Crawler to customize the
// crawling process.
type Options struct {
	// UserAgent is the user-agent value used to make requests to the host.
	UserAgent string

	// RobotUserAgent is the user-agent value of the robot, used to find
	// a matching policy in the robots.txt file of a host. It is not used
	// to make the robots.txt request, only to match a policy.
	// It should always be set to the name of your crawler application so
	// that site owners can configure the robots.txt accordingly.
	RobotUserAgent string

	// MaxVisits is the maximum number of pages visited before
	// automatically stopping the crawler.
	MaxVisits int

	// EnqueueChanBuffer is the size of the buffer for the enqueue channel.
	EnqueueChanBuffer int

	// HostBufferFactor controls the size of the map and channel used
	// internally to manage hosts. If there are 5 different hosts in
	// the initial seeds, and HostBufferFactor is 10, it will create
	// buffered channel of 5 * 10 (50) (and a map of hosts with that
	// initial capacity, though the map will grow as needed).
	HostBufferFactor int

	// CrawlDelay is the default time to wait between requests to a given
	// host. If a specific delay is specified in the relevant robots.txt,
	// then this delay is used instead. Crawl delay can be customized
	// further by implementing the ComputeDelay extender function.
	CrawlDelay time.Duration

	// WorkerIdleTTL is the idle time-to-live allowed for a worker
	// before it is cleared (its goroutine terminated). The crawl
	// delay is not part of idle time, this is specifically the time
	// when the worker is available, but there are no URLs to process.
	WorkerIdleTTL time.Duration

	// SameHostOnly limits the URLs to enqueue only to those targeting
	// the same hosts as the ones from the seed URLs.
	SameHostOnly bool

	// HeadBeforeGet asks the crawler to make a HEAD request before
	// making an eventual GET request. If set to true, the extender
	// method RequestGet is called after the HEAD to control if the
	// GET should be issued.
	HeadBeforeGet bool

	// URLNormalizationFlags controls the normalization of URLs.
	// See the purell package for details.
	URLNormalizationFlags purell.NormalizationFlags

	// LogFlags controls the verbosity of the logger.
	LogFlags LogFlags

	// Extender is the implementation of hooks to use by the crawler.
	Extender Extender
}

// NewOptions creates a new set of Options with default values
// using the provided Extender. The RobotUserAgent option should
// be set to the name of your crawler, it is used to find the matching
// entry in the robots.txt file.
func NewOptions(ext Extender) *Options {
	// Use defaults except for Extender
	return &Options{
		DefaultUserAgent,
		DefaultRobotUserAgent,
		0,
		DefaultEnqueueChanBuffer,
		DefaultHostBufferFactor,
		DefaultCrawlDelay,
		DefaultIdleTTL,
		true,
		false,
		DefaultNormalizationFlags,
		LogError,
		ext,
	}
}
