package gocrawl

import (
	"fmt"
)

type LogFlags uint

// Log levels for the library's logger
const (
	LogError LogFlags = 1 << iota
	LogInfo
	LogEnqueued
	LogIgnored
	LogTrace
	LogNone LogFlags = 0
	LogAll  LogFlags = LogError | LogInfo | LogEnqueued | LogIgnored | LogTrace
)

func getLogFunc(ext Extender, verbosity LogFlags, workerIndex int) func(LogFlags, string, ...interface{}) {
	return func(minLevel LogFlags, format string, vals ...interface{}) {
		if workerIndex > 0 {
			ext.Log(verbosity, minLevel, fmt.Sprintf(fmt.Sprintf("worker %d - %s", workerIndex, format), vals...))
		} else {
			ext.Log(verbosity, minLevel, fmt.Sprintf(format, vals...))
		}
	}
}
