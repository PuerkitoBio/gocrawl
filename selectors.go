package gocrawl

import "github.com/andybalholm/cascadia"

var (
	aHrefMatcher    = cascadia.MustCompile("a[href]")
	baseHrefMatcher = cascadia.MustCompile("base[href]")
)
