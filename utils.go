package gocrawl

import (
	"net/url"
)

// Returns the robots.txt URL for the given host.
func getRobotsTxtUrl(u *url.URL) (*url.URL, error) {
	return u.Parse(robotsTxtPath)
}

// Returns the index of a given string within a slice of strings, or -1.
func indexInStrings(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}
