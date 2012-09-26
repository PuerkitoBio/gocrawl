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
	DefaultRobotUserAgent     string                    = `gocrawl (Googlebot)`
	DefaultCrawlDelay         time.Duration             = 5 * time.Second
	DefaultNormalizationFlags purell.NormalizationFlags = purell.FlagsAllGreedy
)

type Options struct {
	UserAgent             string
	RobotUserAgent        string
	MaxVisits             int
	CrawlDelay            time.Duration // Applied per host
	SameHostOnly          bool
	UrlNormalizationFlags purell.NormalizationFlags
	UrlVisitor            func(*http.Response, *goquery.Document) ([]*url.URL, bool)
	UrlSelector           func(target *url.URL, origin *url.URL, isVisited bool) bool
	Fetcher               Fetcher
	Logger                *log.Logger
	LogFlags              LogFlags
}

func NewOptions(visitor func(*http.Response, *goquery.Document) ([]*url.URL, bool),
	urlSelector func(*url.URL, *url.URL, bool) bool) *Options {

	// Use defaults except for Visitor func
	return &Options{DefaultUserAgent,
		DefaultRobotUserAgent,
		0,
		DefaultCrawlDelay,
		true,
		DefaultNormalizationFlags,
		visitor,
		urlSelector,
		nil,
		log.New(os.Stdout, "gocrawl ", log.LstdFlags|log.Lmicroseconds),
		LogError}
}
