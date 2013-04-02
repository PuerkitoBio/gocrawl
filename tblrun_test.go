package gocrawl

import (
	"fmt"
	"strings"
	"testing"
)

var (
	// Global variable for the in-context of the test case generic assert function.
	assertTrue func(bool, string, ...interface{}) bool
)

func TestRunner(t *testing.T) {
	var singleTC *testCase

	// Check if a single case should run
	for _, tc := range cases {
		if strings.HasPrefix(tc.name, "*") {
			if singleTC == nil {
				singleTC = tc
			} else {
				t.Fatal("multiple test cases for isolated run (prefixed with '*')")
			}
		}
	}
	run := 0
	ign := 0
	if singleTC != nil {
		singleTC.name = singleTC.name[1:]
		run++
		t.Logf("running %s in isolated run...", singleTC.name)
		runTestCase(t, singleTC)
	} else {
		for _, tc := range cases {
			if strings.HasPrefix(tc.name, "!") {
				ign++
				t.Logf("ignoring %s", tc.name[1:])
			} else {
				run++
				t.Logf("running %s...", tc.name)
				runTestCase(t, tc)
			}
		}
	}
	t.Logf("%d test(s) executed, %d test(s) ignored", run, ign)
}

func runTestCase(t *testing.T, tc *testCase) {
	var spy *spyExtender

	// Setup the global assertTrue variable to a closure on this T and test case.
	assertCnt := 0
	assertTrue = func(cond bool, msg string, args ...interface{}) bool {
		assertCnt++
		if !cond {
			t.Errorf("FAIL %s - %s.", tc.name, fmt.Sprintf(msg, args...))
			return false
		}
		return true
	}

	if tc.external != nil {
		// External implementation, do not use this generic runner
		tc.external(t, tc, !strings.HasSuffix(tc.name, ">"))
	} else {
		// Generic runner
		if tc.http {
			ext := new(DefaultExtender)
			spy = newSpy(ext, true)
		} else {
			ff := newFileFetcher()
			spy = newSpy(ff, true)
		}
		if strings.HasSuffix(tc.name, ">") {
			// Debug mode, print log to screen instead of buffer, and log all
			spy.useLogBuffer = false
			tc.opts.LogFlags = LogAll
		}
		tc.opts.Extender = spy
		c := NewCrawlerWithOptions(tc.opts)
		if tc.funcs != nil {
			for emk, f := range tc.funcs {
				spy.setExtensionMethod(emk, f)
			}
		}
		if tc.panics {
			defer assertPanic(tc.name, t)
			assertCnt++
		}

		if err := c.Run(tc.seeds); err != nil && err != ErrMaxVisits {
			t.Errorf("FAIL %s - %s.", tc.name, err)
		}

		for emk, cnt := range tc.asserts {
			assertCallCount(spy, tc.name, emk, cnt, t)
			assertCnt++
		}
		for _, s := range tc.logAsserts {
			if strings.HasPrefix(s, "!") {
				assertIsNotInLog(tc.name, spy.b, s[1:], t)
			} else {
				assertIsInLog(tc.name, spy.b, s, t)
			}
			assertCnt++
		}
		if tc.customAssert != nil {
			tc.customAssert(spy, t)
			assertCnt++
		}
		if assertCnt == 0 {
			t.Errorf("FAIL %s - no asserts.", tc.name)
		}
	}
}
