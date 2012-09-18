package gocrawl

import (
	//"io"
	"net/url"
	//"time"
)

type Options struct {
	BaseUrlPattern       string
	IncludeUrlPatterns   []string
	ExcludeUrlPatterns   []string
	IgnoreQueryKeys      []string
	UrlSelectionCallback func(*url.URL, bool) bool // second arg is wasVisited (true or false)
	RequestHeadBeforeGet bool
	MaxVisits            int
}
