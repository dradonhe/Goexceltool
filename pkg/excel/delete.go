package excel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Dimension 工作表尺寸（1 基口径的行列数）。
type Dimension struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// DeleteOptions 删除行列的配置。
// 行号、列号、自定义范围统一 1 基。
type DeleteOptions struct {
	Sheet      string
	FirstRows  int
	LastRows   int
	CustomRows string
	FirstCols  int
	LastCols   int
	CustomCols string
}

// Noop 判断是否一个删除动作都没选。
func (o DeleteOptions) Noop() bool {
	return o.FirstRows <= 0 && o.LastRows <= 0 && strings.TrimSpace(o.CustomRows) == "" &&
		o.FirstCols <= 0 && o.LastCols <= 0 && strings.TrimSpace(o.CustomCols) == ""
}

// DeleteReport 描述一次删除操作的结果。
type DeleteReport struct {
	Sheet       string    `json:"sheet"`
	Before      Dimension `json:"before"`
	After       Dimension `json:"after"`
	DeletedRows []int     `json:"deletedRows"`
	DeletedCols []int     `json:"deletedCols"`
	Warnings    []string  `json:"warnings,omitempty"`
}

// SheetNames 返回工作簿的全部工作表名。
func SheetNames(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

// SheetDimension 返回指定工作表的行列数；sheet 为空时取第一个工作表。
func SheetDimension(path, sheet string) (Dimension, string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return Dimension{}, "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	sheet, err = pickSheet(f, sheet)
	if err != nil {
		return Dimension{}, "", err
	}

	dim, err := measure(f, sheet)
	if err != nil {
		return Dimension{}, sheet, err
	}
	return dim, sheet, nil
}

// SheetPreview 返回表头（第一行）预览，供前端选择连接键。
func SheetPreview(path, sheet string) (Dimension, []string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return Dimension{}, nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	sheet, err = pickSheet(f, sheet)
	if err != nil {
		return Dimension{}, nil, err
	}

	dim, err := measure(f, sheet)
	if err != nil {
		return Dimension{}, nil, err
	}

	header := make([]string, 0, dim.Cols)
	for c := 1; c <= dim.Cols; c++ {
		name, _ := excelize.CoordinatesToCellName(c, 1)
		v, _ := f.GetCellValue(sheet, name)
		header = append(header, strings.TrimSpace(v))
	}
	return dim, header, nil
}

// ApplyDelete 把 src 复制到 dst 后删除指定行列。
//
// 关键点（相对旧实现的变化）：
//   - 不再 GetRows→NewFile→SetCellValue 重建，而是打开源工作簿、调用 RemoveRow/RemoveCol 后另存。
//     样式、公式、列宽、图片、图表、数据验证、条件格式、其它工作表全部原样保留；
//     excelize 内部会自动调整合并单元格、超链接、条件格式的引用。
//   - 写盘走「临时文件 + 重命名」，覆盖原文件时即使中途失败也不会留下半个文件。
func ApplyDelete(src, dst string, opt DeleteOptions) (*DeleteReport, error) {
	if opt.Noop() {
		return nil, fmt.Errorf("没有选择任何删除操作")
	}

	f, err := excelize.OpenFile(src)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	sheet, err := pickSheet(f, opt.Sheet)
	if err != nil {
		return nil, err
	}

	before, err := measure(f, sheet)
	if err != nil {
		return nil, err
	}
	if before.Rows == 0 || before.Cols == 0 {
		return nil, fmt.Errorf("工作表 %q 是空表，无法删除行列", sheet)
	}

	// 行：首 N + 尾 N + 自定义，去重升序
	rows := append([]int{}, firstN(opt.FirstRows, before.Rows)...)
	rows = append(rows, lastN(opt.LastRows, before.Rows)...)
	rows = append(rows, ParseRowRange(opt.CustomRows, before.Rows)...)
	rows = SortUnique(rows)

	// 列：同样的规则
	cols := append([]int{}, firstN(opt.FirstCols, before.Cols)...)
	cols = append(cols, lastN(opt.LastCols, before.Cols)...)
	cols = append(cols, ParseColumnRange(opt.CustomCols, before.Cols)...)
	cols = SortUnique(cols)

	if len(rows) == 0 && len(cols) == 0 {
		return nil, fmt.Errorf("删除条件没有命中任何行或列，请检查范围输入")
	}
	if len(rows) >= before.Rows && len(cols) >= before.Cols {
		return nil, fmt.Errorf("该配置会删除整个工作表（%d 行 × %d 列），已拒绝", before.Rows, before.Cols)
	}

	return applyDelete(f, sheet, rows, cols, before, dst)
}

// ApplyDeleteIndices 直接按 1 基行号/列号删除，供旧的数组式调用方使用。
func ApplyDeleteIndices(src, dst, sheet string, deleteRows, deleteCols []int) (*DeleteReport, error) {
	f, err := excelize.OpenFile(src)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	name, err := pickSheet(f, sheet)
	if err != nil {
		return nil, err
	}
	before, err := measure(f, name)
	if err != nil {
		return nil, err
	}

	rows := SortUnique(filterRange(deleteRows, before.Rows))
	cols := SortUnique(filterRange(deleteCols, before.Cols))
	if len(rows) == 0 && len(cols) == 0 {
		return nil, fmt.Errorf("没有有效的行号或列号")
	}
	return applyDelete(f, name, rows, cols, before, dst)
}

// applyDelete 在已打开的工作簿上执行删除并另存。
func applyDelete(f *excelize.File, sheet string, rows, cols []int, before Dimension, dst string) (*DeleteReport, error) {
	// 必须从后往前删，否则后面的行/列坐标会失效
	for i := len(rows) - 1; i >= 0; i-- {
		if err := f.RemoveRow(sheet, rows[i]); err != nil {
			return nil, fmt.Errorf("删除第 %d 行失败: %w", rows[i], err)
		}
	}
	for i := len(cols) - 1; i >= 0; i-- {
		name, err := excelize.ColumnNumberToName(cols[i])
		if err != nil {
			return nil, fmt.Errorf("列号 %d 非法: %w", cols[i], err)
		}
		if err := f.RemoveCol(sheet, name); err != nil {
			return nil, fmt.Errorf("删除 %s 列失败: %w", name, err)
		}
	}

	if err := saveAtomic(f, dst); err != nil {
		return nil, err
	}

	return &DeleteReport{
		Sheet:       sheet,
		Before:      before,
		After:       Dimension{Rows: before.Rows - len(rows), Cols: before.Cols - len(cols)},
		DeletedRows: rows,
		DeletedCols: cols,
	}, nil
}

// filterRange 丢弃越界序号并去重（保持 1 基）。
func filterRange(in []int, max int) []int {
	out := make([]int, 0, len(in))
	for _, v := range in {
		if v >= 1 && v <= max {
			out = append(out, v)
		}
	}
	return out
}

// pickSheet 在未指定工作表时回退到第一个，并在指定名称不存在时给出可读的错误。
func pickSheet(f *excelize.File, sheet string) (string, error) {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", fmt.Errorf("工作簿里没有工作表")
	}
	sheet = strings.TrimSpace(sheet)
	if sheet == "" {
		return sheets[0], nil
	}
	for _, s := range sheets {
		if s == sheet {
			return s, nil
		}
	}
	return "", fmt.Errorf("找不到工作表 %q，该文件包含：%s", sheet, strings.Join(sheets, "、"))
}

// measure 统计工作表的实际行列数。
// 优先用维度信息；维度不可信时退化为扫描非空单元格（只在末尾有空列/空行时多花一点时间）。
func measure(f *excelize.File, sheet string) (Dimension, error) {
	if dimStr, err := f.GetSheetDimension(sheet); err == nil && dimStr != "" {
		if rows, cols, ok := parseDimension(dimStr); ok {
			return Dimension{Rows: rows, Cols: cols}, nil
		}
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return Dimension{}, fmt.Errorf("读取工作表 %q 失败: %w", sheet, err)
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	return Dimension{Rows: len(rows), Cols: cols}, nil
}

// parseDimension 解析 "A1:H100" 这类维度串。
func parseDimension(dim string) (rows, cols int, ok bool) {
	parts := strings.Split(dim, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	_, endRow, err1 := excelize.CellNameToCoordinates(parts[1])
	endCol, _, err2 := excelize.CellNameToCoordinates(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return endRow, endCol, true
}

// saveAtomic 先写临时文件再重命名，避免覆盖原文件时产生半截文件。
func saveAtomic(f *excelize.File, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	tmp := filepath.Join(dir, "."+filepath.Base(dst)+".tmp-"+strconv.Itoa(os.Getpid())+"-"+randomSuffix()+".xlsx")
	if err := f.SaveAs(tmp); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("保存到 %s 失败: %w", dst, err)
	}
	return nil
}
