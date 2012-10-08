package gocrawl

import (
	"net/url"
)

func isRobotsTxtUrl(u *url.URL) bool {
	if u == nil {
		return false
	}
	return u.Path == "/robots.txt"
}

func getRobotsTxtUrl(u *url.URL) (*url.URL, error) {
	return u.Parse("/robots.txt")
}

func indexInStrings(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}
