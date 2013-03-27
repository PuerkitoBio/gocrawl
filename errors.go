package gocrawl

import (
	"errors"
)

// Crawl error information.
type CrawlError struct {
	Ctx  *URLContext
	Err  error
	Kind CrawlErrorKind
	msg  string
}

var (
	// The error returned when a redirection is requested, so that the
	// worker knows that this is not an actual Fetch error, but a request to
	// enqueue the redirect-to URL.
	EnqueueRedirectError = errors.New("redirection not followed")
)

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
