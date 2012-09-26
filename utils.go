package gocrawl

import (
	"net/url"
)

func isRobotsTxtUrl(u *url.URL) bool {
	return u.Path == "/robots.txt"
}

func getRobotsTxtUrl(u *url.URL) (*url.URL, error) {
	return u.Parse("/robots.txt")
}
