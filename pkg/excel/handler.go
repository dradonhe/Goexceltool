package excel

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Handler Excel文件处理封装
type Handler struct {
	file *excelize.File
	path string
}

// NewHandler 创建新的Excel处理器
func NewHandler() *Handler {
	return &Handler{}
}

// OpenFile 打开Excel文件
func (h *Handler) OpenFile(filePath string) error {
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}

	h.file = file
	h.path = filePath
	return nil
}

// CreateFile 创建新的Excel文件
func (h *Handler) CreateFile() {
	h.file = excelize.NewFile()
	h.path = ""
}

// Save 保存文件
func (h *Handler) Save() error {
	if h.file == nil {
		return fmt.Errorf("没有打开的文件")
	}

	if h.path == "" {
		return fmt.Errorf("没有指定保存路径")
	}

	return h.file.SaveAs(h.path)
}

// SaveAs 另存为
func (h *Handler) SaveAs(filePath string) error {
	if h.file == nil {
		return fmt.Errorf("没有打开的文件")
	}

	return h.file.SaveAs(filePath)
}

// Close 关闭文件
func (h *Handler) Close() error {
	if h.file != nil {
		return h.file.Close()
	}
	return nil
}

// GetSheetList 获取工作表列表
func (h *Handler) GetSheetList() []string {
	if h.file == nil {
		return []string{}
	}
	return h.file.GetSheetList()
}

// GetSheetRows 获取工作表行数据
func (h *Handler) GetSheetRows(sheetName string) ([][]string, error) {
	if h.file == nil {
		return nil, fmt.Errorf("文件未打开")
	}

	return h.file.GetRows(sheetName)
}

// GetSheetDimensions 获取工作表维度
func (h *Handler) GetSheetDimensions(sheetName string) (int, int, error) {
	if h.file == nil {
		return 0, 0, fmt.Errorf("文件未打开")
	}

	rows, err := h.GetSheetRows(sheetName)
	if err != nil {
		return 0, 0, err
	}

	if len(rows) == 0 {
		return 0, 0, nil
	}

	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	return len(rows), maxCols, nil
}

// DeleteRows 删除指定行
func (h *Handler) DeleteRows(sheetName string, rowNumbers []int) error {
	if h.file == nil {
		return fmt.Errorf("文件未打开")
	}

	// 排序并去重（降序排列以便正确删除）
	sort.Sort(sort.Reverse(sort.IntSlice(rowNumbers)))

	for _, rowNum := range rowNumbers {
		if err := h.file.RemoveRow(sheetName, rowNum); err != nil {
			// 如果删除失败，尝试其他方式
			h.shiftRowsUp(sheetName, rowNum)
		}
	}

	return nil
}

// DeleteColumns 删除指定列
func (h *Handler) DeleteColumns(sheetName string, colIndices []int) error {
	if h.file == nil {
		return fmt.Errorf("文件未打开")
	}

	// 排序并去重（降序排列）
	sort.Sort(sort.Reverse(sort.IntSlice(colIndices)))

	for _, colIndex := range colIndices {
		colName, err := excelize.ColumnNumberToName(colIndex + 1)
		if err != nil {
			continue
		}

		// 删除整列
		if err := h.file.RemoveCol(sheetName, colName); err != nil {
			// 如果删除失败，清空列内容
			h.clearColumn(sheetName, colIndex)
		}
	}

	return nil
}

// SetCellValue 设置单元格值
func (h *Handler) SetCellValue(sheetName, cell string, value interface{}) error {
	if h.file == nil {
		return fmt.Errorf("文件未打开")
	}

	return h.file.SetCellValue(sheetName, cell, value)
}

// GetCellValue 获取单元格值
func (h *Handler) GetCellValue(sheetName, cell string) (string, error) {
	if h.file == nil {
		return "", fmt.Errorf("文件未打开")
	}

	return h.file.GetCellValue(sheetName, cell)
}

// CopySheet 复制工作表
func (h *Handler) CopySheet(fromSheet, toSheet string) error {
	if h.file == nil {
		return fmt.Errorf("文件未打开")
	}
	fromIdx, err := h.file.GetSheetIndex(fromSheet)
	if err != nil || fromIdx <= 0 {
		return fmt.Errorf("找不到源工作表: %v", err)
	}
	toIdx, err := h.file.GetSheetIndex(toSheet)
	if err != nil || toIdx <= 0 {
		return fmt.Errorf("找不到目标工作表: %v", err)
	}
	return h.file.CopySheet(fromIdx, toIdx)
}

// RenameSheet 重命名工作表
func (h *Handler) RenameSheet(oldName, newName string) error {
	if h.file == nil {
		return fmt.Errorf("文件未打开")
	}

	return h.file.SetSheetName(oldName, newName)
}

// shiftRowsUp 将指定行之后的所有行上移
func (h *Handler) shiftRowsUp(sheetName string, startRow int) error {
	rows, err := h.GetSheetRows(sheetName)
	if err != nil {
		return err
	}

	if startRow > len(rows) {
		return nil
	}

	// 从startRow开始，将后续行的内容上移一行
	for i := startRow; i < len(rows); i++ {
		nextRow := i + 1
		if nextRow <= len(rows) {
			for j, cellValue := range rows[nextRow-1] {
				cellName, _ := excelize.CoordinatesToCellName(j+1, i)
				h.file.SetCellValue(sheetName, cellName, cellValue)
			}
		}
	}

	// 清除最后一行
	lastRow := len(rows)
	for j := 0; j < len(rows[0]); j++ {
		cellName, _ := excelize.CoordinatesToCellName(j+1, lastRow)
		h.file.SetCellValue(sheetName, cellName, "")
	}

	return nil
}

// clearColumn 清空指定列的内容
func (h *Handler) clearColumn(sheetName string, colIndex int) error {
	rows, err := h.GetSheetRows(sheetName)
	if err != nil {
		return err
	}

	for i := 0; i < len(rows); i++ {
		cellName, _ := excelize.CoordinatesToCellName(colIndex+1, i+1)
		h.file.SetCellValue(sheetName, cellName, "")
	}

	return nil
}

// Helper functions for column/row operations

// ParseRowRange 解析行范围字符串 (如 "1,3,5-10")
func ParseRowRange(rangeStr string, maxRows int) []int {
	var rows []int

	if rangeStr == "" {
		return rows
	}

	parts := strings.Split(strings.ReplaceAll(rangeStr, " ", ""), ",")

	for _, part := range parts {
		if strings.Contains(part, "-") {
			// 范围格式
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(rangeParts[0])
				end, err2 := strconv.Atoi(rangeParts[1])

				if err1 == nil && err2 == nil && start > 0 && end <= maxRows {
					for i := start; i <= end; i++ {
						rows = append(rows, i)
					}
				}
			}
		} else {
			// 单个行号
			rowNum, err := strconv.Atoi(part)
			if err == nil && rowNum > 0 && rowNum <= maxRows {
				rows = append(rows, rowNum)
			}
		}
	}

	return removeDuplicatesAndSort(rows)
}

// ParseColumnRange 解析列范围字符串 (如 "A,C,E-G")
func ParseColumnRange(rangeStr string, maxCols int) []int {
	var cols []int

	if rangeStr == "" {
		return cols
	}

	parts := strings.Split(strings.ToUpper(strings.ReplaceAll(rangeStr, " ", "")), ",")

	for _, part := range parts {
		if strings.Contains(part, "-") {
			// 范围格式
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start := colToNum(rangeParts[0])
				end := colToNum(rangeParts[1])

				if start > 0 && end <= maxCols {
					for i := start; i <= end; i++ {
						cols = append(cols, i-1) // 转为0基索引
					}
				}
			}
		} else {
			// 单个列
			colNum := colToNum(part)
			if colNum > 0 && colNum <= maxCols {
				cols = append(cols, colNum-1) // 转为0基索引
			}
		}
	}

	return removeDuplicatesAndSort(cols)
}

// colToNum 列字母转数字
func colToNum(colStr string) int {
	result := 0
	for _, char := range colStr {
		if char >= 'A' && char <= 'Z' {
			result = result*26 + int(char-'A'+1)
		}
	}
	return result
}

// removeDuplicatesAndSort 去重并排序
func removeDuplicatesAndSort(slice []int) []int {
	seen := make(map[int]bool)
	var result []int

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	sort.Ints(result)
	return result
}

// ValidateSheetName 验证工作表名称
func ValidateSheetName(name string) bool {
	if name == "" || len(name) > 31 {
		return false
	}

	// 检查非法字符
	illegalChars := []string{"\\", "/", "?", "*", "[", "]"}
	for _, char := range illegalChars {
		if strings.Contains(name, char) {
			return false
		}
	}

	return true
}

// MergeSameHeader merges multiple Excel files using the header of mainFile as canonical order.
func MergeSameHeader(mainFile string, inputFiles []string, sheetName string, outputPath string) error {
	if len(inputFiles) == 0 {
		return fmt.Errorf("至少需要一个输入文件")
	}

	// Open main file and read header
	mainF, err := excelize.OpenFile(mainFile)
	if err != nil {
		return fmt.Errorf("打开主文件失败: %v", err)
	}
	defer mainF.Close()

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
	_ = out.DeleteSheet("Sheet1")
	outSheetName := strings.TrimSpace(sheetName)
	if outSheetName == "" {
		outSheetName = "Sheet1"
	}
	out.NewSheet(outSheetName)

	// Write header
	for ci, c := range header {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		if err := out.SetCellValue(outSheetName, cell, c); err != nil {
			return fmt.Errorf("写入输出表头失败: %v", err)
		}
	}

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
	// Write main file's data rows (preserve main rows in output)
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

	// Iterate input files
	for _, fpath := range inputFiles {
		f, err := excelize.OpenFile(fpath)
		if err != nil {
			return fmt.Errorf("打开输入文件失败(%s): %v", fpath, err)
		}
		// Ensure the sheet exists; if not, skip this file
		if _, err := f.GetSheetIndex(sheetName); err != nil {
			f.Close()
			continue
		}
		rows, err := f.GetRows(sheetName)
		f.Close()
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}

		curHeader := rows[0]
		headerIndex := make(map[string]int, len(curHeader))
		for idx, h := range curHeader {
			headerIndex[h] = idx
		}

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

	if err := out.SaveAs(outputPath); err != nil {
		return fmt.Errorf("保存输出文件失败: %v", err)
	}
	return nil
}
