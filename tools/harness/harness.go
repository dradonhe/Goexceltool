// 开发用小工具：在指定目录里挑若干 xlsx 做一次同构合并，用于快速验证。
//
//	go run ./tools/harness -dir ./testdata
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"

	excel "exceltool/pkg/excel"
)

func main() {
	dir := flag.String("dir", "testdata", "包含 xlsx 文件的目录")
	flag.Parse()

	if err := runMerge(*dir); err != nil {
		log.Fatalf("合并失败: %v", err)
	}
}

func runMerge(dir string) error {
	files, err := collectXLSX(dir)
	if err != nil {
		return err
	}
	if len(files) < 2 {
		fmt.Printf("目录 %s 下不足 2 个 xlsx，跳过\n", dir)
		return nil
	}

	mainFile := files[0]
	inputs := files[1:]
	sheet := commonSheet(files)
	fmt.Printf("主文件: %s\nSheet: %s\n输入文件: %d 个\n", mainFile, sheet, len(inputs))

	out := filepath.Join(dir, "merged_output_"+time.Now().Format("20060102_150405")+".xlsx")
	report, err := excel.MergeFiles(mainFile, inputs, sheet, out)
	if err != nil {
		return err
	}

	fmt.Printf("合并完成：%d 行 × %d 列，来源 %d 个文件\n输出：%s\n", report.Rows, report.Cols, report.Files, out)
	for _, s := range report.Skipped {
		fmt.Printf("  跳过: %s\n", s)
	}
	return nil
}

func collectXLSX(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".xlsx" {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out, nil
}

// commonSheet 选一个在主文件里存在、且在最多文件里出现的表名。
func commonSheet(files []string) string {
	count := map[string]int{}
	var mainSheets []string

	for i, p := range files {
		f, err := excelize.OpenFile(p)
		if err != nil {
			continue
		}
		sheets := f.GetSheetList()
		if i == 0 {
			mainSheets = sheets
		}
		for _, s := range sheets {
			count[s]++
		}
		f.Close()
	}

	best, bestCount := "Sheet1", 0
	for _, s := range mainSheets {
		if count[s] > bestCount {
			best, bestCount = s, count[s]
		}
	}
	return best
}
