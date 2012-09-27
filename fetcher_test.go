package gocrawl

import (
	"net/http"
	"net/url"
	"os"
	"path"
)

type fileFetcher struct {
	basePath string
}

func NewFileFetcher(basePath string) *fileFetcher {
	return &fileFetcher{basePath}
}

func (this *fileFetcher) Fetch(u *url.URL, userAgent string) (*http.Response, error) {
	var res *http.Response = new(http.Response)
	var req *http.Request = new(http.Request)

	// Prepare the pseudo-request
	req.Method = "GET"
	req.URL = u
	req.Header.Add("User-Agent", userAgent)

	// Open the file specified as path in u, relative to testdata/[host]/
	f, e := os.Open(path.Join(this.basePath, u.Host, u.Path))
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
