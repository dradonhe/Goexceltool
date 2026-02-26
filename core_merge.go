package main

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// MergeSameHeader merges multiple Excel files that share the same header structure.
// It uses the header of the provided mainFile as the canonical column order
// and aligns all input files to that order. Missing columns are filled with empty strings.
// sheetName specifies which sheet to operate on in all input files.
// outputPath is the path of the generated merged Excel file.
func MergeSameHeader(mainFile string, inputFiles []string, sheetName string, outputPath string) error {
	if len(inputFiles) == 0 {
		return fmt.Errorf("至少需要一个输入文件")
	}

	// Open main file and read header
	mainF, err := excelize.OpenFile(mainFile)
	if err != nil {
		return fmt.Errorf("打开主文件失败: %v", err)
	}
	defer mainF.Close() // in case of early return, ensure close

	mainRows, err := mainF.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("读取主文件数据失败: %v", err)
	}
	if len(mainRows) == 0 {
		return fmt.Errorf("主文件没有数据")
	}
	header := mainRows[0]
	if len(header) == 0 {
		return fmt.Errorf("主文件表头为空")
	}

	// Prepare output workbook
	out := excelize.NewFile()
	// Remove default Sheet1 if present
	out.DeleteSheet("Sheet1")
	outSheetName := strings.TrimSpace(sheetName)
	if outSheetName == "" {
		outSheetName = "Sheet1"
	}
	out.NewSheet(outSheetName)

	// Write header to output (row 1)
	for ci, c := range header {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		if err := out.SetCellValue(outSheetName, cell, c); err != nil {
			return fmt.Errorf("写入输出表头失败: %v", err)
		}
	}

	// Helper to write a aligned row against final header
	writeAlignedRow := func(row []string, rowIndex int) error {
		for ci := 0; ci < len(header); ci++ {
			cell, _ := excelize.CoordinatesToCellName(ci+1, rowIndex)
			var val string
			if ci < len(row) {
				val = row[ci]
			} else {
				val = ""
			}
			if err := out.SetCellValue(outSheetName, cell, val); err != nil {
				return err
			}
		}
		return nil
	}

	outRow := 2
	// Write main file's data rows into output
	for r := 1; r < len(mainRows); r++ {
		row := mainRows[r]
		aligned := make([]string, len(header))
		for i := 0; i < len(header); i++ {
			if i < len(row) {
				aligned[i] = row[i]
			} else {
				aligned[i] = ""
			}
		}
		if err := writeAlignedRow(aligned, outRow); err != nil {
			return fmt.Errorf("写入主文件行失败: %v", err)
		}
		outRow++
	}

	// Iterate over each input file and align to header
	for _, fpath := range inputFiles {
		f, err := excelize.OpenFile(fpath)
		if err != nil {
			return fmt.Errorf("打开输入文件失败(%s): %v", fpath, err)
		}
		// Ensure the sheet exists in this file; if not, skip this input file
		if _, err := f.GetSheetIndex(sheetName); err != nil {
			f.Close()
			continue
		}
		rows, err := f.GetRows(sheetName)
		f.Close()
		if err != nil {
			// skip files where rows cannot be read
			continue
		}
		if len(rows) == 0 {
			continue
		}
		// Build a header index map for the current file
		curHeader := rows[0]
		headerIndex := make(map[string]int, len(curHeader))
		for idx, h := range curHeader {
			headerIndex[h] = idx
		}

		// Process data rows (skip header)
		for r := 1; r < len(rows); r++ {
			aligned := make([]string, len(header))
			for i := 0; i < len(header); i++ {
				h := header[i]
				if pos, ok := headerIndex[h]; ok && pos < len(rows[r]) {
					aligned[i] = rows[r][pos]
				} else {
					aligned[i] = ""
				}
			}
			if err := writeAlignedRow(aligned, outRow); err != nil {
				return fmt.Errorf("写入输出行失败: %v", err)
			}
			outRow++
		}
	}

	// Save output
	if err := out.SaveAs(outputPath); err != nil {
		return fmt.Errorf("保存输出文件失败: %v", err)
	}
	return nil
}
