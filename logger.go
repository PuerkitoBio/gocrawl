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
	LogTrace
	LogNone LogFlags = 0
)

func getLogFunc(logger *log.Logger, level LogFlags, workerIndex int) func(LogFlags, string, ...interface{}) {
	return func(minLevel LogFlags, format string, vals ...interface{}) {
		if logger != nil {
			if workerIndex > 0 {
				if level&minLevel == minLevel {
					logger.Printf(fmt.Sprintf("Worker %d - %s", workerIndex, format), vals...)
				}
			} else {
				if level&minLevel == minLevel {
					logger.Printf(format, vals...)
				}
			}
		}
	}
}
