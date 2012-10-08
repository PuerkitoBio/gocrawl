package gocrawl

import (
	"fmt"
	"log"
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

func getLogFunc(logger *log.Logger, level LogFlags, workerIndex int) func(LogFlags, string, ...interface{}) {
	return func(minLevel LogFlags, format string, vals ...interface{}) {
		if logger != nil {
			if workerIndex > 0 {
				if level&minLevel == minLevel {
					logger.Printf(fmt.Sprintf("worker %d - %s", workerIndex, format), vals...)
				}
			} else {
				if level&minLevel == minLevel {
					logger.Printf(format, vals...)
				}
			}
		}
	}
}
