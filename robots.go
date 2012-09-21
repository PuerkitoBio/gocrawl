package gocrawl

import (
	"fmt"
	"github.com/temoto/robotstxt.go"
	"io/ioutil"
	"net/url"
)

// Caching of robots.txt data per host
var cache = make(map[string]*robotstxt.RobotsData, 10)

// It is assumed the URL is already normalized.
func isUrlAllowedPerRobots(u *url.URL, robotUserAgent string) (bool, error) {
	rd, e := getHostRobots(u.Host)
	if e != nil {
		return false, e
	}

	return rd.TestAgent(u.String(), robotUserAgent)
}

func getHostRobots(host string) (*robotstxt.RobotsData, error) {
	data, ok := cache[host]
	if !ok {
		// Data not available for this host, request the file
		if res, e := httpClient.Get(fmt.Sprintf("http://%s/robots.txt", host)); e != nil {
			return nil, e
		} else {
			defer res.Body.Close()

			if body, e := ioutil.ReadAll(res.Body); e != nil {
				return nil, e
			} else if data, e = robotstxt.FromResponseBytes(res.StatusCode, body, false); e != nil {
				return nil, e
			} else {
				cache[host] = data
			}
		}
	}
	return data, nil
}
