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
	Logger                *log.Logger
	LogFlags              LogFlags
	Extender              Extender
}

// Options constructor based on a visitor and filter callback functions.
func NewOptions(visitor func(*http.Response, *goquery.Document) ([]*url.URL, bool),
	filter func(*url.URL, *url.URL, bool) (bool, int)) *Options {

	return NewOptionsWithExtender(&DefaultExtender{
		visitor,
		filter,
	})
}

func NewOptionsWithExtender(ext Extender) *Options {
	// Use defaults except for Extender
	return &Options{DefaultUserAgent,
		DefaultRobotUserAgent,
		0,
		DefaultCrawlDelay,
		DefaultIdleTTL,
		true,
		DefaultNormalizationFlags,
		log.New(os.Stdout, "gocrawl ", log.LstdFlags|log.Lmicroseconds),
		LogError,
		ext}
}
