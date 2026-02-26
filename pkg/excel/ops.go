package excel

import (
	"fmt"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

// DeleteRowsAndCols reads filePath's sheetName, deletes specified rows and columns (1-based indices),
// and writes result to outputPath. Returns error on failure.
func DeleteRowsAndCols(filePath, sheetName string, deleteRows, deleteCols []int, outputPath string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("读取数据失败: %v", err)
	}

	// delete rows
	// convert deleteRows 1-based to 0-based set
	rowSet := make(map[int]bool)
	for _, r := range deleteRows {
		if r > 0 {
			rowSet[r-1] = true
		}
	}
	var afterRows [][]string
	for i, row := range rows {
		if !rowSet[i] {
			afterRows = append(afterRows, row)
		}
	}

	// delete columns
	colSet := make(map[int]bool)
	for _, c := range deleteCols {
		if c > 0 {
			colSet[c-1] = true
		}
	}
	finalRows := make([][]string, len(afterRows))
	for i, row := range afterRows {
		var nr []string
		for j, cell := range row {
			if !colSet[j] {
				nr = append(nr, cell)
			}
		}
		finalRows[i] = nr
	}

	// write to output
	out := excelize.NewFile()
	out.DeleteSheet("Sheet1")
	out.NewSheet(sheetName)
	for rIdx, row := range finalRows {
		for cIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
			out.SetCellValue(sheetName, cell, val)
		}
	}

	// ensure outputPath has filename
	if outputPath == "" {
		outputPath = filepath.Join(".", "result.xlsx")
	}

	if err := out.SaveAs(outputPath); err != nil {
		return fmt.Errorf("保存文件失败: %v", err)
	}

	return nil
}
