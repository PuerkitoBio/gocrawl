package gocrawl

import (
	"fmt"
	"log"
)

type LogLevel uint

// Log levels for the library's logger
const (
	LogError LogLevel = 1 << iota
	LogInfo
	LogTrace
	LogNone LogLevel = 0
)

func getLogFunc(logger *log.Logger, level LogLevel, workerIndex int) func(LogLevel, string, ...interface{}) {
	return func(minLevel LogLevel, format string, vals ...interface{}) {
		if workerIndex > 0 {
			if level|minLevel == minLevel {
				logger.Printf(fmt.Sprintf("Worker %d - %s", workerIndex, format), vals...)
			}
		} else {
			if level|minLevel == minLevel {
				logger.Printf(format, vals...)
			}
		}
	}
}
