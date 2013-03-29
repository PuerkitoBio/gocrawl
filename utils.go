package gocrawl

// Returns the index of a given string within a slice of strings, or -1.
func indexInStrings(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}
