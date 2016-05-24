package gocrawl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/PuerkitoBio/purell"
)

func TestRedirectURLContext(t *testing.T) {
	mustParse := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			t.Fatalf("failed to parse URL %s", s)
		}
		return u
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/p1":
			http.Redirect(w, r, "/p2", http.StatusTemporaryRedirect)
		case "/p2":
			http.Redirect(w, r, "/p3", http.StatusTemporaryRedirect)
		default:
			fmt.Fprint(w, "ok")
		}
	}))
	defer srv.Close()

	spy := newSpy(new(DefaultExtender), true)
	c := NewCrawlerWithOptions(NewOptions(spy))
	c.Options.CrawlDelay = time.Millisecond
	c.Options.UserAgent = "test"
	c.Options.URLNormalizationFlags = purell.FlagAddTrailingSlash
	if err := c.Run(srv.URL + "/p1"); err != nil {
		t.Fatalf("run failed with %v", err)
	}

	assertCallCount(spy, "fetch", eMKFetch, 4, t)   // 3 URLs, robots.txt
	assertCallCount(spy, "visit", eMKVisit, 1, t)   // single visit for p3
	assertCallCount(spy, "filter", eMKFilter, 3, t) // 3 URLs

	callCounts := []struct {
		ctx       *URLContext
		filterCnt int
		fetchCnt  int
		visitCnt  int
	}{
		{
			&URLContext{
				url:           mustParse(srv.URL + "/p1"),
				normalizedURL: mustParse(srv.URL + "/p1/"),
			}, 1, 1, 0,
		},
		{
			&URLContext{
				url:                 mustParse(srv.URL + "/p2"),
				normalizedURL:       mustParse(srv.URL + "/p2/"),
				sourceURL:           mustParse(srv.URL + "/p1"),
				normalizedSourceURL: mustParse(srv.URL + "/p1/"),
			}, 1, 1, 0,
		},
		{
			&URLContext{
				url:                 mustParse(srv.URL + "/p3"),
				normalizedURL:       mustParse(srv.URL + "/p3/"),
				sourceURL:           mustParse(srv.URL + "/p1"),
				normalizedSourceURL: mustParse(srv.URL + "/p1/"),
			}, 1, 1, 1,
		},
	}
	for i, cc := range callCounts {
		if n := spy.getCalledWithCount(eMKFilter, cc.ctx, false); n != cc.filterCnt {
			t.Errorf("%d: want %d filter call for %s, got %d", i, cc.filterCnt, cc.ctx.url, n)
		}
		if n := spy.getCalledWithCount(eMKFetch, cc.ctx, "test", false); n != cc.fetchCnt {
			t.Errorf("%d: want %d fetch call for %s, got %d", i, cc.fetchCnt, cc.ctx.url, n)
		}
		if n := spy.getCalledWithCount(eMKVisit, cc.ctx, ignore, ignore); n != cc.visitCnt {
			t.Errorf("%d: want %d visit call for %s, got %d", i, cc.visitCnt, cc.ctx.url, n)
		}
	}
}
