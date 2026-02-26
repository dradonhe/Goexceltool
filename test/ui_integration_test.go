package goexcelt_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/user/go-exceltool/go-exceltool/pkg/excel"
	"github.com/xuri/excelize/v2"
)

// SimulateUITrigger 调用 MergeSameHeader，模拟 UI 层触发操作
func SimulateUITrigger(mainFile string, inputFiles []string, sheetName string, outputPath string) error {
	// In a real UI this call might be invoked in a goroutine after user confirmation
	return excel.MergeSameHeader(mainFile, inputFiles, sheetName, outputPath)
}

// gatherRowsCols 返回指定文件和 sheet 的行数与列数
func gatherRowsCols(path, sheet string) (rows int, cols int, err error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	rs, err := f.GetRows(sheet)
	if err != nil {
		return 0, 0, err
	}
	maxCols := 0
	for _, r := range rs {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}
	return len(rs), maxCols, nil
}

// findExcelfilesDir 查找项目中的 excelfiles 目录
func findExcelfilesDir() string {
	candidates := []string{
		filepath.Join("..", "excelfiles"),
		filepath.Join(".", "..", "excelfiles"),
		filepath.Join("..", "..", "excelfiles"),
		`E:\\Texcel\\excelfiles`,
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

func TestUISimulatedRuns(t *testing.T) {
	// when running tests for this package the working dir is the package folder `.../test`
	// so write the log file into the current package folder.
	logPath := "UItestlog.md"
	// overwrite log
	lf, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("无法创建日志文件: %v", err)
	}
	defer lf.Close()

	exceldir := findExcelfilesDir()
	if exceldir == "" {
		t.Fatalf("找不到 excelfiles 目录")
	}

	entries, err := os.ReadDir(exceldir)
	if err != nil {
		t.Fatalf("读取 excelfiles 失败: %v", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".xlsx") {
			continue
		}
		// skip very large files to keep integration test time reasonable
		fi, err := os.Stat(filepath.Join(exceldir, e.Name()))
		if err == nil {
			if fi.Size() > 10*1024*1024 { // skip files >10MB
				fmt.Printf("跳过大文件以节省时间: %s (%.1fMB)\n", e.Name(), float64(fi.Size())/1024.0/1024.0)
				continue
			}
		}
		files = append(files, filepath.Join(exceldir, e.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("在 %s 中未找到任何 xlsx 文件", exceldir)
	}

	// For each main file, test a limited number of combinations to keep runtime bounded
	testCount := 0
	maxTests := 200
	for mi, main := range files {
		// get sheets in main
		mf, err := excelize.OpenFile(main)
		if err != nil {
			fmt.Fprintf(lf, "# 主文件读取失败: %s -> %v\n", main, err)
			continue
		}
		mainsheets := mf.GetSheetList()
		mf.Close()

		// build other files slice
		var others []string
		for i, f := range files {
			if i == mi {
				continue
			}
			others = append(others, f)
		}

		n := len(others)
		// iterate over subsets but cap total tests
		maxMask := 1 << n
		for mask := 1; mask < maxMask; mask++ {
			if testCount >= maxTests {
				break
			}
			var inputs []string
			for i := 0; i < n; i++ {
				if mask&(1<<i) != 0 {
					inputs = append(inputs, others[i])
				}
			}

			// for each sheet in main, run the merge
			for _, sheet := range mainsheets {
				testCount++
				fmt.Fprintf(lf, "## Test #%d\n", testCount)
				fmt.Fprintf(lf, "- Time: %s\n", time.Now().Format(time.RFC3339))
				fmt.Fprintf(lf, "- Main: %s\n", main)
				fmt.Fprintf(lf, "- Inputs: %s\n", strings.Join(inputs, ", "))
				fmt.Fprintf(lf, "- Sheet: %s\n", sheet)

				// record main rows/cols
				mr, mc, merr := gatherRowsCols(main, sheet)
				if merr != nil {
					fmt.Fprintf(lf, "- Main sheet read error: %v\n", merr)
				} else {
					fmt.Fprintf(lf, "- Main rows: %d, cols: %d\n", mr, mc)
				}

				// record inputs rows/cols
				for _, in := range inputs {
					ir, ic, ierr := gatherRowsCols(in, sheet)
					if ierr != nil {
						fmt.Fprintf(lf, "- Input %s sheet read error: %v\n", in, ierr)
					} else {
						fmt.Fprintf(lf, "- Input %s rows: %d, cols: %d\n", in, ir, ic)
					}
				}

				// output path in temp dir
				outdir := t.TempDir()
				outpath := filepath.Join(outdir, fmt.Sprintf("merged_output_%d.xlsx", testCount))

				// simulate UI trigger
				err := SimulateUITrigger(main, inputs, sheet, outpath)
				if err != nil {
					fmt.Fprintf(lf, "- Merge error: %v\n\n", err)
					continue
				}

				// read output rows/cols
				or, oc, oerr := gatherRowsCols(outpath, sheet)
				if oerr != nil {
					fmt.Fprintf(lf, "- Output read error: %v\n\n", oerr)
				} else {
					fmt.Fprintf(lf, "- Output: %s\n", outpath)
					fmt.Fprintf(lf, "- Output rows: %d, cols: %d\n\n", or, oc)
				}
				if testCount >= maxTests {
					break
				}
			}
			if testCount >= maxTests {
				break
			}
		}
	}
	lf.Sync()
	t.Logf("UI 模拟测试完成，生成 %d 个用例，日志写入 %s", testCount, filepath.Join("test", logPath))

	// --- 执行删除操作的 UI 模拟并写入同一目录下的 UIDeleteLog.md ---
	deleteLogPath := filepath.Join("UIDeleteLog.md")
	df, derr := os.Create(deleteLogPath)
	if derr == nil {
		defer df.Close()
		fmt.Fprintf(df, "# 删除操作 UI 模拟测试\n\n")
		tIdx := 0
		for _, f := range files {
			xf, err := excelize.OpenFile(f)
			if err != nil {
				fmt.Fprintf(df, "- 打开文件失败: %s -> %v\n", f, err)
				continue
			}
			sheets := xf.GetSheetList()
			xf.Close()
			if len(sheets) == 0 {
				continue
			}
			sheet := sheets[0]
			// 删除第一行和第一列
			tIdx++
			outdir := t.TempDir()
			outpath := filepath.Join(outdir, fmt.Sprintf("delete_out_%d.xlsx", tIdx))
			err = excel.DeleteRowsAndCols(f, sheet, []int{1}, []int{1}, outpath)
			if err != nil {
				fmt.Fprintf(df, "## Delete Test #%d\n- File: %s\n- Sheet: %s\n- Error: %v\n\n", tIdx, f, sheet, err)
				continue
			}
			mr, mc, _ := gatherRowsCols(f, sheet)
			or, oc, _ := gatherRowsCols(outpath, sheet)
			fmt.Fprintf(df, "## Delete Test #%d\n- File: %s\n- Sheet: %s\n- Main rows: %d, cols: %d\n- Output: %s\n- Output rows: %d, cols: %d\n\n", tIdx, f, sheet, mr, mc, outpath, or, oc)
		}
		df.Sync()
		t.Logf("删除 UI 模拟测试完成，日志写入 %s", filepath.Join("test", deleteLogPath))
	} else {
		t.Logf("无法创建删除日志文件: %v", derr)
	}
}

// Additionally test delete rows/cols UI action for each file
func TestUIDeleteAction(t *testing.T) {
	logPath := "UIDeleteLog.md"
	lf, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("无法创建删除日志文件: %v", err)
	}
	defer lf.Close()

	exceldir := findExcelfilesDir()
	if exceldir == "" {
		t.Fatalf("找不到 excelfiles 目录")
	}

	entries, err := os.ReadDir(exceldir)
	if err != nil {
		t.Fatalf("读取 excelfiles 失败: %v", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".xlsx") {
			files = append(files, filepath.Join(exceldir, e.Name()))
		}
	}
	if len(files) == 0 {
		t.Fatalf("未找到任何 xlsx 文件")
	}

	testIdx := 0
	for _, f := range files {
		// pick first sheet
		xf, err := excelize.OpenFile(f)
		if err != nil {
			fmt.Fprintf(lf, "- 打开文件失败: %s -> %v\n", f, err)
			continue
		}
		sheets := xf.GetSheetList()
		xf.Close()
		if len(sheets) == 0 {
			continue
		}
		sheet := sheets[0]

		// define deletion: delete first 1 row and first 1 column if present
		testIdx++
		outdir := t.TempDir()
		outpath := filepath.Join(outdir, fmt.Sprintf("delete_out_%d.xlsx", testIdx))
		err = excel.DeleteRowsAndCols(f, sheet, []int{1}, []int{1}, outpath)
		if err != nil {
			fmt.Fprintf(lf, "## Delete Test #%d\n- File: %s\n- Sheet: %s\n- Error: %v\n\n", testIdx, f, sheet, err)
			continue
		}
		mr, mc, _ := gatherRowsCols(f, sheet)
		or, oc, _ := gatherRowsCols(outpath, sheet)
		fmt.Fprintf(lf, "## Delete Test #%d\n- File: %s\n- Sheet: %s\n- Main rows: %d, cols: %d\n- Output: %s\n- Output rows: %d, cols: %d\n\n", testIdx, f, sheet, mr, mc, outpath, or, oc)
	}
	lf.Sync()
	t.Logf("删除 UI 模拟测试完成，日志写入 %s", filepath.Join("test", "UIDeleteLog.md"))
}
