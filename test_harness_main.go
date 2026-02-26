//go:build test_harness
// +build test_harness

package main

import "log"

// Entry point for the test harness when building with -tags test_harness
func main() {
	if err := RunTestMerge(); err != nil {
		log.Fatalf("测试失败: %v", err)
	}
}
