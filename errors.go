package gocrawl

import (
	"errors"
)

var (
	// ErrEnqueueRedirect is returned when a redirection is requested, so that the
	// worker knows that this is not an actual Fetch error, but a request to
	// enqueue the redirect-to URL.
	ErrEnqueueRedirect = errors.New("redirection not followed")

	// ErrMaxVisits is returned when the maximum number of visits, as specified by the
	// Options field MaxVisits, is reached.
	ErrMaxVisits = errors.New("the maximum number of visits is reached")

	// ErrInterrupted is returned when the crawler is manually stopped
	// (via a call to Stop).
	ErrInterrupted = errors.New("interrupted")
)

// CrawlErrorKind indicated the kind of crawling error.
type CrawlErrorKind uint8

// The various kinds of crawling errors.
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

// String returns the string representation of the error kind.
func (cek CrawlErrorKind) String() string {
	return lookupCek[cek]
}

// CrawlError contains information about the crawling error.
type CrawlError struct {
	// The URL Context where the error occurred.
	Ctx *URLContext

	// The underlying error.
	Err error

	// The error kind.
	Kind CrawlErrorKind

	msg string
}

// Error implements of the error interface for CrawlError.
func (ce CrawlError) Error() string {
	if ce.Err != nil {
		return ce.Err.Error()
	}
	return ce.msg
}

// Create a new CrawlError based on a source error.
func newCrawlError(ctx *URLContext, e error, kind CrawlErrorKind) *CrawlError {
	return &CrawlError{ctx, e, kind, ""}
}

// Create a new CrawlError with the specified message.
func newCrawlErrorMessage(ctx *URLContext, msg string, kind CrawlErrorKind) *CrawlError {
	return &CrawlError{ctx, nil, kind, msg}
}
