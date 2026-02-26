//go:build test_harness
// +build test_harness

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"
)

// Test harness for MergeSameHeader using test files under excelfiles.
// This does not modify the original input files.
func RunTestMerge() error {
	// Path containing test Excel files
	testDir := `E:\Texcel\excelfiles`

	// Collect all .xlsx files in testDir (non-recursive preference; fallback to recursive if needed)
	var files []string
	// Try non-recursive first
	if entries, err := os.ReadDir(testDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if filepath.Ext(e.Name()) == ".xlsx" {
				files = append(files, filepath.Join(testDir, e.Name()))
			}
		}
	}

	// If not enough, fallback to recursive search
	if len(files) < 2 {
		filepath.WalkDir(testDir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(p) == ".xlsx" {
				files = append(files, p)
			}
			return nil
		})
	}

	if len(files) < 2 {
		log.Println("测试数据不足：在 excelfiles 目录下需要至少 2 个 .xlsx 测试文件")
		fmt.Println("[Test] 未能找到足够的测试文件，测试中止。")
		return nil
	}

	// Use the first file as mainFile, rest as inputFiles
	mainFile := files[0]
	inputFiles := files[1:]

	// Determine a sheet name that is common across files (prefer Sheet1). Build frequency map.
	sheetName := "Sheet1"
	sheetCount := make(map[string]int)
	allFiles := append([]string{mainFile}, inputFiles...)
	for _, p := range allFiles {
		if f, err := excelize.OpenFile(p); err == nil {
			for _, s := range f.GetSheetList() {
				sheetCount[s]++
			}
			f.Close()
		}
	}

	// Inspect main file's sheets and prefer a sheet that exists in main file and in other files.
	mainSheets := []string{}
	if f, err := excelize.OpenFile(mainFile); err == nil {
		mainSheets = f.GetSheetList()
		f.Close()
	}

	// If main has Sheet1 and it's common, pick it
	if contains(mainSheets, "Sheet1") && sheetCount["Sheet1"] >= 1 {
		sheetName = "Sheet1"
		fmt.Printf("[Test] Selected sheet: %s (from main file)\n", sheetName)
	} else {
		// find the most common sheet that main also has
		best := ""
		bestCount := 0
		for _, s := range mainSheets {
			if c := sheetCount[s]; c > bestCount {
				best = s
				bestCount = c
			}
		}
		if best != "" {
			sheetName = best
			fmt.Printf("[Test] Selected sheet: %s (appears in %d files)\n", sheetName, bestCount)
		} else if len(mainSheets) > 0 {
			sheetName = mainSheets[0]
			fmt.Printf("[Test] Fallback to main's first sheet: %s\n", sheetName)
		}
	}

	// 生成输出路径
	ts := time.Now().Format("20060102_150405")
	outputPath := filepath.Join(testDir, "merged_output_"+ts+".xlsx")

	if err := MergeSameHeader(mainFile, inputFiles, sheetName, outputPath); err != nil {
		return fmt.Errorf("同构合并失败: %v", err)
	}

	fmt.Printf("[Test] 合并完成，输出文件：%s\n", outputPath)
	return nil
}

// contains helper
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
