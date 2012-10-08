package gocrawl

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Default options
const (
	DefaultUserAgent          string                    = `Mozilla/5.0 (Windows NT 6.1; rv:15.0) Gecko/20120716 Firefox/15.0a2`
	DefaultRobotUserAgent     string                    = `Googlebot (gocrawl v0.1)`
	DefaultCrawlDelay         time.Duration             = 5 * time.Second
	DefaultIdleTTL            time.Duration             = 10 * time.Second
	DefaultNormalizationFlags purell.NormalizationFlags = purell.FlagsAllGreedy
)

// The Options available to control and customize the crawling process.
type Options struct {
	UserAgent             string
	RobotUserAgent        string
	MaxVisits             int
	CrawlDelay            time.Duration // Applied per host
	WorkerIdleTTL         time.Duration
	SameHostOnly          bool
	URLNormalizationFlags purell.NormalizationFlags
	URLVisitor            func(*http.Response, *goquery.Document) ([]*url.URL, bool)
	URLSelector           func(target *url.URL, origin *url.URL, isVisited bool) bool
	Fetcher               Fetcher
	Logger                *log.Logger
	LogFlags              LogFlags
}

// Options constructor based on a visitor and selector callback functions.
func NewOptions(visitor func(*http.Response, *goquery.Document) ([]*url.URL, bool),
	urlSelector func(*url.URL, *url.URL, bool) bool) *Options {

	// Use defaults except for Visitor func
	return &Options{DefaultUserAgent,
		DefaultRobotUserAgent,
		0,
		DefaultCrawlDelay,
		DefaultIdleTTL,
		true,
		DefaultNormalizationFlags,
		visitor,
		urlSelector,
		nil,
		log.New(os.Stdout, "gocrawl ", log.LstdFlags|log.Lmicroseconds),
		LogError}
}
