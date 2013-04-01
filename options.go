package gocrawl

import (
	"github.com/PuerkitoBio/purell"
	"time"
)

// Default options
const (
	DefaultUserAgent          string                    = `Mozilla/5.0 (Windows NT 6.1; rv:15.0) Gecko/20120716 Firefox/15.0a2`
	DefaultRobotUserAgent     string                    = `Googlebot (gocrawl v0.4)`
	DefaultEnqueueChanBuffer  int                       = 100
	DefaultHostBufferFactor   int                       = 10
	DefaultCrawlDelay         time.Duration             = 5 * time.Second
	DefaultIdleTTL            time.Duration             = 10 * time.Second
	DefaultNormalizationFlags purell.NormalizationFlags = purell.FlagsAllGreedy
)

// The Options available to control and customize the crawling process.
type Options struct {
	UserAgent             string
	RobotUserAgent        string
	MaxVisits             int
	EnqueueChanBuffer     int
	HostBufferFactor      int
	CrawlDelay            time.Duration // Applied per host
	WorkerIdleTTL         time.Duration
	SameHostOnly          bool
	HeadBeforeGet         bool
	URLNormalizationFlags purell.NormalizationFlags
	LogFlags              LogFlags
	Extender              Extender
}

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
