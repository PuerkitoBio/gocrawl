package gocrawl

import (
	"github.com/PuerkitoBio/purell"
	"net/url"
)

const (
	robotsTxtPath = "/robots.txt"
)

type U map[*url.URL]interface{}
type S map[string]interface{}

type URLContext struct {
	HeadBeforeGet bool
	State         interface{}

	// Internal fields, available through getters
	url                 *url.URL
	normalizedURL       *url.URL
	origin              EnqueueOrigin
	sourceURL           *url.URL
	normalizedSourceURL *url.URL
}

func (this *URLContext) URL() *url.URL {
	return this.url
}

func (this *URLContext) NormalizedURL() *url.URL {
	return this.normalizedURL
}

func (this *URLContext) Origin() EnqueueOrigin {
	return this.origin
}

func (this *URLContext) SourceURL() *url.URL {
	return this.sourceURL
}

func (this *URLContext) NormalizedSourceURL() *url.URL {
	return this.normalizedSourceURL
}

func (this *URLContext) IsRobotsURL() {
	if this.normalizedURL == nil {
		return false
	}
	return this.normalizedURL.Path == robotsTxtPath
}

func (this *URLContext) GetRobotsURL() (*url.URL, error) {
	return this.normalizedURL.Parse(robotsTxtPath)
}

func (this *Crawler) toURLContexts(raw interface{}, src *url.URL, orig EnqueueOrigin) []*URLContext {
	var res []*URLContext

	switch v := raw.(type) {
	case string:
		// Convert a single string URL to an URLContext
		ctx, err := stringToURLContext(v, src, orig)
		if err != nil {
			// TODO : Log error and ignore URL?
		}
		res = []*URLContext{ctx}

	case []string:
		// Convert all strings to URLContexts
		res = make([]*URLContext, len(v))
		for i, s := range v {
			ctx, err := this.stringToURLContext(s, src, orig)
			if err != nil {
				// TODO : Log error and ignore URL?
			}
			res[i] = ctx
		}

	case *url.URL:
		res = []*URLContext{urlToURLContext(v, src, orig)}

	case []*url.URL:
		res = make([]*URLContext, len(v))
		for i, u := range v {
			res[i] = this.urlToURLContext(u, src, orig)
		}

	case S, map[string]interface{}:

	case U, map[*url.URL]interface{}:
	}
}

func (this *Crawler) stringToURLContext(str string, src *url.URL, orig EnqueueOrigin) (*URLContext, error) {
	u, err := url.Parse(str)
	if err != nil {
		return err
	}
	return this.urlToURLContext(u, src, orig), nil
}

func (this *Crawler) urlToURLContext(u, src *url.URL, orig EnqueueOrigin) *URLContext {
	rawU := *u
	purell.NormalizeURL(u, this.Options.URLNormalizationFlags)
	rawSrc := *src
	purell.NormalizeURL(src, this.Options.URLNormalizationFlags)

	return &URLContext{
		this.Options.HeadBeforeGet,
		nil,
		&rawU,
		u,
		orig,
		rawSrc,
		src,
	}
}
