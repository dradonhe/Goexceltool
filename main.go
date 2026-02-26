package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func main() {
	fmt.Println("=== Excel行列删除功能实际测试 ===")

	// 测试配置
	testFile := `e:\Texcel\excelfiles\核定待办0101-0131.xlsx`
	sheetName := "任务待办导出"

	fmt.Printf("处理文件: %s\n", testFile)
	fmt.Printf("Sheet名称: %s\n", sheetName)

	// 检查文件存在性
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		fmt.Printf("❌ 文件不存在: %s\n", testFile)
		return
	}

	// 执行删除操作
	resultFile, err := deleteRowsAndCols(testFile, sheetName, []int{1, 2}, []int{1})
	if err != nil {
		fmt.Printf("❌ 处理失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 处理成功！\n")
	fmt.Printf("结果文件: %s\n", resultFile)

	// 验证结果
	verifyFile(resultFile, sheetName)
}

func deleteRowsAndCols(filePath, sheetName string, deleteRows, deleteCols []int) (string, error) {
	// 读取原始文件
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer f.Close()

	// 获取原始数据
	originalRows, err := f.GetRows(sheetName)
	if err != nil {
		return "", fmt.Errorf("读取数据失败: %v", err)
	}

	originalRowCount := len(originalRows)
	originalColCount := getMaxColCount(originalRows)

	fmt.Printf("原始数据: %d行 x %d列\n", originalRowCount, originalColCount)

	// 删除指定行
	newRows := deleteSpecifiedRows(originalRows, deleteRows)
	afterRowsCount := len(newRows)

	// 删除指定列
	finalRows := deleteSpecifiedCols(newRows, deleteCols)
	afterColsCount := getMaxColCount(finalRows)

	fmt.Printf("处理后数据: %d行 x %d列\n", afterRowsCount, afterColsCount)
	fmt.Printf("删除统计: %d行, %d列\n", len(deleteRows), len(deleteCols))

	// 生成新文件名
	dir := filepath.Dir(filePath)
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	ext := filepath.Ext(filePath)
	timestamp := time.Now().Format("20060102_150405")
	newFileName := fmt.Sprintf("%s_result_%s%s", base, timestamp, ext)
	resultPath := filepath.Join(dir, newFileName)

	// 创建新文件
	newFile := excelize.NewFile()
	newFile.DeleteSheet("Sheet1")
	newFile.NewSheet(sheetName)

	// 写入处理后的数据
	for rowIndex, row := range finalRows {
		for colIndex, cellValue := range row {
			cellName, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			newFile.SetCellValue(sheetName, cellName, cellValue)
		}
	}

	// 保存文件
	err = newFile.SaveAs(resultPath)
	if err != nil {
		return "", fmt.Errorf("保存文件失败: %v", err)
	}

	return resultPath, nil
}

func deleteSpecifiedRows(rows [][]string, deleteRows []int) [][]string {
	// 转换为0-based索引
	rowSet := make(map[int]bool)
	for _, row := range deleteRows {
		rowSet[row-1] = true
	}

	// 保留未被删除的行
	var result [][]string
	for i, row := range rows {
		if !rowSet[i] {
			result = append(result, row)
		}
	}

	return result
}

func deleteSpecifiedCols(rows [][]string, deleteCols []int) [][]string {
	if len(deleteCols) == 0 {
		return rows
	}

	// 转换为0-based索引
	colSet := make(map[int]bool)
	for _, col := range deleteCols {
		colSet[col-1] = true
	}

	// 删除指定列
	result := make([][]string, len(rows))
	for i, row := range rows {
		var newRow []string
		for j, cell := range row {
			if !colSet[j] {
				newRow = append(newRow, cell)
			}
		}
		result[i] = newRow
	}

	return result
}

func verifyFile(filePath, sheetName string) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		fmt.Printf("验证失败 - 无法打开文件: %v\n", err)
		return
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		fmt.Printf("验证失败 - 无法读取数据: %v\n", err)
		return
	}

	rowCount := len(rows)
	colCount := getMaxColCount(rows)

	fmt.Printf("\n=== 验证结果 ===\n")
	fmt.Printf("文件: %s\n", filePath)
	fmt.Printf("Sheet: %s\n", sheetName)
	fmt.Printf("最终行数: %d\n", rowCount)
	fmt.Printf("最终列数: %d\n", colCount)
	fmt.Printf("✅ 验证完成\n")
}

func getMaxColCount(rows [][]string) int {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}
