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
	sourceURL           *url.URL
	normalizedSourceURL *url.URL
}

func (this *URLContext) URL() *url.URL {
	return this.url
}

func (this *URLContext) NormalizedURL() *url.URL {
	return this.normalizedURL
}

func (this *URLContext) SourceURL() *url.URL {
	return this.sourceURL
}

func (this *URLContext) NormalizedSourceURL() *url.URL {
	return this.normalizedSourceURL
}

func (this *URLContext) IsRobotsURL() bool {
	if this.normalizedURL == nil {
		return false
	}
	return this.normalizedURL.Path == robotsTxtPath
}

func (this *URLContext) GetRobotsURLCtx() (*URLContext, error) {
	robUrl, err := this.normalizedURL.Parse(robotsTxtPath)
	if err != nil {
		return nil, err
	}
	return &URLContext{
		false, // Never request HEAD before GET for robots.txt
		nil,   // Always nil state
		robUrl,
		robUrl,         // Normalized is same as raw
		this.sourceURL, // Source and normalized source is same as for current context
		this.normalizedSourceURL,
	}, nil
}

func (this *Crawler) toURLContexts(raw interface{}, src *url.URL) []*URLContext {
	var res []*URLContext

	switch v := raw.(type) {
	case string:
		// Convert a single string URL to an URLContext
		ctx, err := stringToURLContext(v, src)
		if err != nil {
			this.Options.Extender.Error(newCrawlError(e, CekParseSeed, nil))
			this.logFunc(LogError, "ERROR parsing seed %s", s)
		} else {
			res = []*URLContext{ctx}
		}

	case []string:
		// Convert all strings to URLContexts
		res = make([]*URLContext, 0, len(v))
		for _, s := range v {
			ctx, err := this.stringToURLContext(s, src)
			if err != nil {
				this.Options.Extender.Error(newCrawlError(e, CekParseSeed, nil))
				this.logFunc(LogError, "ERROR parsing seed %s", s)
			} else {
				res = append(res, ctx)
			}
		}

	case *url.URL:
		res = []*URLContext{urlToURLContext(v, src)}

	case []*url.URL:
		res = make([]*URLContext, 0, len(v))
		for _, u := range v {
			res = append(res, this.urlToURLContext(u, src))
		}

	case map[string]interface{}: // TODO : Idem for type S
		res = make([]*URLContext, 0, len(v))
		for s, st := range v {
			ctx, err := this.stringToURLContext(s, src)
			if err != nil {
				this.Options.Extender.Error(newCrawlError(e, CekParseSeed, nil))
				this.logFunc(LogError, "ERROR parsing seed %s", s)
			} else {
				ctx.State = st
				res = append(res, ctx)
			}
		}

	case map[*url.URL]interface{}: // TODO : Idem for type U
		res = make([]*URLContext, 0, len(v))
		for u, st := range v {
			ctx := this.urlToURLContext(u, src)
			ctx.State = st
			res = append(res, ctx)
		}
	}
	return res
}

func (this *Crawler) stringToURLContext(str string, src *url.URL) (*URLContext, error) {
	u, err := url.Parse(str)
	if err != nil {
		return err
	}
	return this.urlToURLContext(u, src), nil
}

func (this *Crawler) urlToURLContext(u, src *url.URL) *URLContext {
	rawU := *u
	purell.NormalizeURL(u, this.Options.URLNormalizationFlags)
	rawSrc := *src
	purell.NormalizeURL(src, this.Options.URLNormalizationFlags)

	return &URLContext{
		this.Options.HeadBeforeGet,
		nil,
		&rawU,
		u,
		rawSrc,
		src,
	}
}
