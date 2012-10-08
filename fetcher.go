package gocrawl

import (
	"net/http"
	"net/url"
)

// The Fetcher interface defines the Fetch method needed to return an HTTP response.
type Fetcher interface {
	Fetch(*url.URL, string) (*http.Response, error)
}

// The default fetcher's implementation
type defaultFetcher struct {
	http.Client
}

// Fetch requests the specified URL using the given user agent string. It uses
// Go's default http Client instance.
func (this *defaultFetcher) Fetch(u *url.URL, userAgent string) (*http.Response, error) {
	// Prepare the request with the right user agent
	req, e := http.NewRequest("GET", u.String(), nil)
	if e != nil {
		return nil, e
	}
	req.Header["User-Agent"] = []string{userAgent}
	return this.Do(req)
}
