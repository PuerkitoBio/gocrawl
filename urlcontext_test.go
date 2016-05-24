package gocrawl

import (
	"reflect"
	"testing"

	"github.com/PuerkitoBio/purell"
)

func TestCloneForRedirect(t *testing.T) {
	c := NewCrawler(&DefaultExtender{})
	c.Options.URLNormalizationFlags = purell.FlagAddTrailingSlash

	// test 1 : redirect from a seed URL p1 to p2
	ctx1, _ := c.stringToURLContext("http://localhost/p1", nil)
	ctx1.HeadBeforeGet = true
	ctx1.State = 1
	p2, _ := ctx1.URL().Parse("/p2")
	ctx2 := ctx1.cloneForRedirect(p2, c.Options.URLNormalizationFlags)

	var got string
	wantSrc := "http://localhost/p1"
	if src := ctx2.SourceURL(); src != nil {
		got = src.String()
	}
	if got != wantSrc {
		t.Errorf("want source %s, got %s", wantSrc, got)
	}
	wantNSrc := "http://localhost/p1/"
	if nsrc := ctx2.NormalizedSourceURL(); nsrc != nil {
		got = nsrc.String()
	}
	if got != wantNSrc {
		t.Errorf("want normalized source %s, got %s", wantNSrc, got)
	}
	wantU := "http://localhost/p2"
	if u := ctx2.URL(); u != nil {
		got = u.String()
	}
	if got != wantU {
		t.Errorf("want URL %s, got %s", wantU, got)
	}
	wantNU := "http://localhost/p2/"
	if nu := ctx2.NormalizedURL(); nu != nil {
		got = nu.String()
	}
	if got != wantNU {
		t.Errorf("want normalized URL %s, got %s", wantNU, got)
	}
	if !reflect.DeepEqual(ctx2.State, int(1)) {
		t.Errorf("want state %v, got %v", 1, ctx2.State)
	}
	if !ctx2.HeadBeforeGet {
		t.Error("want HeadBeforeGet to be true")
	}

	// test 2: redirect again from p2 to p3, should keep p1 as source
	p3, _ := ctx2.URL().Parse("/p3")
	ctx3 := ctx2.cloneForRedirect(p3, c.Options.URLNormalizationFlags)
	if src := ctx3.SourceURL(); src != nil {
		got = src.String()
	}
	if got != wantSrc {
		t.Errorf("want source %s, got %s", wantSrc, got)
	}
	if nsrc := ctx3.NormalizedSourceURL(); nsrc != nil {
		got = nsrc.String()
	}
	if got != wantNSrc {
		t.Errorf("want normalized source %s, got %s", wantNSrc, got)
	}

	wantU = "http://localhost/p3"
	if u := ctx3.URL(); u != nil {
		got = u.String()
	}
	if got != wantU {
		t.Errorf("want URL %s, got %s", wantU, got)
	}
	wantNU = "http://localhost/p3/"
	if nu := ctx3.NormalizedURL(); nu != nil {
		got = nu.String()
	}
	if got != wantNU {
		t.Errorf("want normalized URL %s, got %s", wantNU, got)
	}

	if !reflect.DeepEqual(ctx3.State, int(1)) {
		t.Errorf("want state %v, got %v", 1, ctx3.State)
	}
	if !ctx3.HeadBeforeGet {
		t.Error("want HeadBeforeGet to be true")
	}
}
