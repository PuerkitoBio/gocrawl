package gocrawl

import (
	"errors"
)

var (
	// The error returned when a redirection is requested, so that the
	// worker knows that this is not an actual Fetch error, but a request to
	// enqueue the redirect-to URL.
	ErrEnqueueRedirect = errors.New("redirection not followed")

	// The error returned when the maximum number of visits, as specified by the
	// Options field MaxVisits, is reached.
	ErrMaxVisits = errors.New("the maximum number of visits is reached")

	ErrInterrupted = errors.New("interrupted")
)

// Enum indicating the kind of the crawling error.
type CrawlErrorKind uint8

const (
	CekFetch CrawlErrorKind = iota
	CekParseRobots
	CekHttpStatusCode
	CekReadBody
	CekParseBody
	CekParseURL
	CekProcessLinks
	CekParseRedirectURL
)

var (
	lookupCek = [...]string{
		CekFetch:            "Fetch",
		CekParseRobots:      "ParseRobots",
		CekHttpStatusCode:   "HttpStatusCode",
		CekReadBody:         "ReadBody",
		CekParseBody:        "ParseBody",
		CekParseURL:         "ParseURL",
		CekProcessLinks:     "ProcessLinks",
		CekParseRedirectURL: "ParseRedirectURL",
	}
)

func (this CrawlErrorKind) String() string {
	return lookupCek[this]
}

// Crawl error information.
type CrawlError struct {
	Ctx  *URLContext
	Err  error
	Kind CrawlErrorKind
	msg  string
}

// Implementation of the error interface.
func (this CrawlError) Error() string {
	if this.Err != nil {
		return this.Err.Error()
	}
	return this.msg
}

// Create a new CrawlError based on a source error.
func newCrawlError(ctx *URLContext, e error, kind CrawlErrorKind) *CrawlError {
	return &CrawlError{ctx, e, kind, ""}
}

// Create a new CrawlError with the specified message.
func newCrawlErrorMessage(ctx *URLContext, msg string, kind CrawlErrorKind) *CrawlError {
	return &CrawlError{ctx, nil, kind, msg}
}
