package gocrawl

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const (
	DefaultTestCrawlDelay = 100 * time.Millisecond
)

func assertIsInLog(nm string, buf bytes.Buffer, s string, t *testing.T) {
	assertLog(nm, buf, s, true, t)
}

func assertIsNotInLog(nm string, buf bytes.Buffer, s string, t *testing.T) {
	assertLog(nm, buf, s, false, t)
}

func assertLog(nm string, buf bytes.Buffer, s string, in bool, t *testing.T) {
	if lg := buf.String(); strings.Contains(lg, s) != in {
		if in {
			t.Errorf("FAIL %s - expected log to contain '%s'.", nm, s)
		} else {
			t.Errorf("FAIL %s - expected log NOT to contain '%s'.", nm, s)
		}
		t.Logf("Log is: %s", lg)
	}
}

func assertCallCount(spy *spyExtender, nm string, key extensionMethodKey, i int, t *testing.T) {
	cnt := spy.getCallCount(key)
	if cnt != i {
		t.Errorf("FAIL %s - expected %d calls to %s, got %d.", nm, i, key, cnt)
	}
}

func assertPanic(nm string, t *testing.T) {
	if e := recover(); e == nil {
		t.Errorf("FAIL %s - expected a panic.", nm)
	}
}
