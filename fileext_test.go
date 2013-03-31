package gocrawl

import (
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	FileFetcherBasePath = "./testdata/"
)

// The file fetcher, that loads URLs from files in the testdata/ directory.
type fileFetcherExtender struct {
	*DefaultExtender
}

// FileFetcher constructor, creates the internal default implementation
func newFileFetcher() *fileFetcherExtender {
	return &fileFetcherExtender{new(DefaultExtender)}
}

// FileFetcher's Fetch() implementation
func (this *fileFetcherExtender) Fetch(ctx *URLContext, userAgent string, headRequest bool) (*http.Response, error) {
	var res *http.Response = new(http.Response)
	var req *http.Request
	var e error

	if req, e = http.NewRequest("GET", ctx.url.String(), nil); e != nil {
		panic(e)
	}

	// Prepare the pseudo-request
	req.Header.Add("User-Agent", userAgent)

	// Open the file specified as path in u, relative to testdata/[host]/
	host := ctx.url.Host
	if strings.HasPrefix(host, "www.") {
		host = host[4:]
	}
	f, e := os.Open(path.Join(FileFetcherBasePath, host, ctx.url.Path))
	if e != nil {
		// Treat errors as 404s - file not found
		res.Status = "404 Not Found"
		res.StatusCode = 404
	} else {
		res.Status = "200 OK"
		res.StatusCode = 200
		res.Body = f
	}
	res.Request = req

	return res, e
}
