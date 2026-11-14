package excel

import (
	"fmt"
	"path/filepath"
)

// DeleteRowsAndCols 保留旧接口：按 1 基行号/列号删除后另存到 outputPath。
// 内部实现已改为「打开源文件 → RemoveRow/RemoveCol → 另存」，样式与公式不再丢失。
func DeleteRowsAndCols(filePath, sheetName string, deleteRows, deleteCols []int, outputPath string) error {
	if outputPath == "" {
		outputPath = filepath.Join(".", "result.xlsx")
	}
	_, err := ApplyDeleteIndices(filePath, outputPath, sheetName, deleteRows, deleteCols)
	if err != nil {
		return fmt.Errorf("删除行列失败: %w", err)
	}
	return nil
}
