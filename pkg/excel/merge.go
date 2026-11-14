package excel

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// MergeReport 同构合并的结果。
type MergeReport struct {
	Header   []string `json:"header"`
	Rows     int      `json:"rows"`     // 不含表头
	Cols     int      `json:"cols"`     //
	Files    int      `json:"files"`    // 实际参与合并的文件数
	Skipped  []string `json:"skipped"`  // 被跳过的文件及原因
	Warnings []string `json:"warnings"` //
}

// MergeSameHeader 保留旧签名，行为与 MergeFiles 一致。
func MergeSameHeader(mainFile string, inputFiles []string, sheetName string, outputPath string) error {
	_, err := MergeFiles(mainFile, inputFiles, sheetName, outputPath)
	return err
}

// MergeFiles 把多个同构文件按「主文件表头」对齐后合并成一个新工作簿。
//
// 与旧实现的区别：
//   - 输入用 Rows() 流式读取，输出用 StreamWriter 流式写入，内存占用与总行数基本无关
//     （旧实现 GetRows 会把每个文件的所有单元格一次性读进内存）
//   - 表头为空的列/重复列会被跳过并记录警告，不会静默错位
//   - 输出表名与 "Sheet1" 同名时不再误删自己的表
func MergeFiles(mainFile string, inputFiles []string, sheetName string, outputPath string) (*MergeReport, error) {
	if strings.TrimSpace(mainFile) == "" {
		return nil, fmt.Errorf("请先选择主文件")
	}
	if len(inputFiles) == 0 {
		return nil, fmt.Errorf("至少需要一个待合并的输入文件")
	}

	mainF, err := excelize.OpenFile(mainFile)
	if err != nil {
		return nil, fmt.Errorf("打开主文件失败: %w", err)
	}
	defer mainF.Close()

	sheet, err := pickSheet(mainF, sheetName)
	if err != nil {
		return nil, fmt.Errorf("主文件: %w", err)
	}

	header, err := readHeader(mainF, sheet)
	if err != nil {
		return nil, err
	}
	if len(header) == 0 {
		return nil, fmt.Errorf("主文件 %q 的表头为空", sheet)
	}

	report := &MergeReport{Header: header}
	// 表头去重：同名表头只保留第一次出现的位置
	keep := make([]int, 0, len(header))
	seen := make(map[string]bool, len(header))
	for i, h := range header {
		name := strings.TrimSpace(h)
		if name == "" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("主文件表头第 %d 列为空，已忽略", i+1))
			continue
		}
		if seen[name] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("主文件表头 %q 重复出现，只保留第一列", name))
			continue
		}
		seen[name] = true
		keep = append(keep, i)
	}
	if len(keep) == 0 {
		return nil, fmt.Errorf("主文件表头全部为空，无法对齐")
	}
	report.Cols = len(keep)

	// 输出工作簿
	out := excelize.NewFile()
	defer out.Close()
	outSheet := sheet
	created := false
	if outSheet != "Sheet1" {
		if _, err := out.NewSheet(outSheet); err != nil {
			return nil, fmt.Errorf("创建输出工作表失败: %w", err)
		}
		created = true
	}
	sw, err := out.NewStreamWriter(outSheet)
	if err != nil {
		return nil, fmt.Errorf("初始化流式写入失败: %w", err)
	}

	// 写表头
	headRow := make([]interface{}, len(keep))
	for i, src := range keep {
		headRow[i] = header[src]
	}
	if err := sw.SetRow("A1", headRow); err != nil {
		return nil, fmt.Errorf("写入表头失败: %w", err)
	}

	outRow := 2
	// 主文件数据行：位置原样保留（用 keep 去掉空/重复列）
	if err := eachRow(mainF, sheet, func(row []string) error {
		values := make([]interface{}, len(keep))
		for i, src := range keep {
			if src < len(row) {
				values[i] = row[src]
			} else {
				values[i] = ""
			}
		}
		if err := sw.SetRow(rowCell(outRow), values); err != nil {
			return err
		}
		outRow++
		report.Rows++
		return nil
	}); err != nil {
		return nil, fmt.Errorf("读取主文件数据失败: %w", err)
	}
	report.Files = 1

	// 其余文件：按表头名对齐
	for _, path := range inputFiles {
		added, skip, warns, err := appendFile(sw, outSheet, path, sheet, header, keep, &outRow)
		if err != nil {
			return nil, err
		}
		report.Warnings = append(report.Warnings, warns...)
		if skip != "" {
			report.Skipped = append(report.Skipped, skip)
			continue
		}
		report.Rows += added
		report.Files++
	}

	if err := sw.Flush(); err != nil {
		return nil, fmt.Errorf("写出合并结果失败: %w", err)
	}
	if created {
		if err := out.DeleteSheet("Sheet1"); err != nil {
			return nil, fmt.Errorf("清理默认工作表失败: %w", err)
		}
	}
	out.SetActiveSheet(0)
	if err := saveAtomic(out, outputPath); err != nil {
		return nil, err
	}
	return report, nil
}

// appendFile 把一个输入文件按主文件表头写进输出，返回新增行数。
func appendFile(sw *excelize.StreamWriter, outSheet, path, sheet string, header []string, keep []int, outRow *int) (added int, skipped string, warnings []string, err error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return 0, fmt.Sprintf("%s（打开失败: %v）", path, err), nil, nil
	}
	defer f.Close()

	if _, err := f.GetSheetIndex(sheet); err != nil {
		return 0, fmt.Sprintf("%s（缺少工作表 %q）", path, sheet), nil, nil
	}

	// 输入文件的表头 → 列号
	curHeader, err := readHeader(f, sheet)
	if err != nil {
		return 0, fmt.Sprintf("%s（表头读取失败: %v）", path, err), nil, nil
	}
	pos := make(map[string]int, len(curHeader))
	for i, h := range curHeader {
		name := strings.TrimSpace(h)
		if _, ok := pos[name]; !ok && name != "" {
			pos[name] = i
		}
	}

	missing := make([]string, 0, 2)
	for _, src := range keep {
		if _, ok := pos[strings.TrimSpace(header[src])]; !ok {
			missing = append(missing, header[src])
		}
	}
	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("%s 缺少列：%s（已留空）", path, strings.Join(missing, "、")))
	}

	if err := eachRow(f, sheet, func(row []string) error {
		values := make([]interface{}, len(keep))
		for i, src := range keep {
			name := strings.TrimSpace(header[src])
			if at, ok := pos[name]; ok && at < len(row) {
				values[i] = row[at]
			} else {
				values[i] = ""
			}
		}
		if err := sw.SetRow(rowCell(*outRow), values); err != nil {
			return err
		}
		*outRow = *outRow + 1
		added++
		return nil
	}); err != nil {
		return added, fmt.Sprintf("%s（数据读取失败: %v）", path, err), warnings, nil
	}
	return added, "", warnings, nil
}

// readHeader 读取第一行作为表头。
func readHeader(f *excelize.File, sheet string) ([]string, error) {
	rows, err := f.Rows(sheet)
	if err != nil {
		return nil, fmt.Errorf("读取工作表 %q 失败: %w", sheet, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Error(); err != nil {
			return nil, fmt.Errorf("读取工作表 %q 失败: %w", sheet, err)
		}
		return nil, nil
	}
	row, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("读取表头失败: %w", err)
	}
	return row, nil
}

// eachRow 从第二行开始逐行回调（跳过表头，流式）。
func eachRow(f *excelize.File, sheet string, fn func([]string) error) error {
	rows, err := f.Rows(sheet)
	if err != nil {
		return err
	}
	defer rows.Close()

	first := true
	for rows.Next() {
		if first { // 跳过表头
			first = false
			continue
		}
		row, err := rows.Columns()
		if err != nil {
			return err
		}
		if err := fn(row); err != nil {
			return err
		}
	}
	return rows.Error()
}
