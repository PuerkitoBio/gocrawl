package gocrawl

import (
	"fmt"
	"testing"
)

// Quick test to check for performance of "find in slice" vs "find in map"
// Map is an obvious and dominant winner, even for small N (map is a constant
// ~100ns/op).
const (
	sliceVsMapN         = 10000
	sliceVsMapFindIndex = 8000
)

func BenchmarkSliceFind(b *testing.B) {
	b.StopTimer()
	var s = make([]string, sliceVsMapN, sliceVsMapN)
	for i := 0; i < sliceVsMapN; i++ {
		s[i] = fmt.Sprintf("http://www.host.com/path/p%d", i)
	}
	var f = s[sliceVsMapFindIndex]
	var found bool
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range s {
			if v == f {
				found = true
				break
			}
		}
	}
	b.Logf("Found: %v", found)
}

func BenchmarkMapFind(b *testing.B) {
	b.StopTimer()
	var m = make(map[string]byte, sliceVsMapN)
	for i := 0; i < sliceVsMapN; i++ {
		m[fmt.Sprintf("http://www.host.com/path/p%d", i)] = '0'
	}
	var f = fmt.Sprintf("http://www.host.com/path/p%d", sliceVsMapFindIndex)
	var ok bool
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, ok = m[f]
	}
	b.Logf("Found: %v", ok)
}
