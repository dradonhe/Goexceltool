package excel

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Handler 是工作簿的薄封装，供需要连续操作的场景使用。
//
// 说明：批量业务（删除行列 / 合并 / 连接）都走 delete.go、merge.go、join.go 的
// 函数式接口，它们对同一份文件只做一次开-改-存，避免 Handler 长期持有句柄。
type Handler struct {
	file *excelize.File
	path string
}

// NewHandler 创建处理器。
func NewHandler() *Handler { return &Handler{} }

// OpenFile 打开工作簿。
func (h *Handler) OpenFile(filePath string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	h.file = f
	h.path = filePath
	return nil
}

// CreateFile 新建空工作簿。
func (h *Handler) CreateFile() {
	h.file = excelize.NewFile()
	h.path = ""
}

// Save 保存到当前路径。
func (h *Handler) Save() error {
	if h.file == nil {
		return fmt.Errorf("没有打开的文件")
	}
	if h.path == "" {
		return fmt.Errorf("没有指定保存路径")
	}
	return h.file.Save()
}

// SaveAs 另存为。
func (h *Handler) SaveAs(filePath string) error {
	if h.file == nil {
		return fmt.Errorf("没有打开的文件")
	}
	return h.file.SaveAs(filePath)
}

// Close 关闭工作簿。
func (h *Handler) Close() error {
	if h.file == nil {
		return nil
	}
	err := h.file.Close()
	h.file = nil
	return err
}

// Sheets 返回工作表名列表。
func (h *Handler) Sheets() []string {
	if h.file == nil {
		return nil
	}
	return h.file.GetSheetList()
}

// Dimension 返回指定工作表的行列数；sheet 为空时用第一个工作表。
func (h *Handler) Dimension(sheet string) (Dimension, error) {
	if h.file == nil {
		return Dimension{}, fmt.Errorf("文件未打开")
	}
	name, err := pickSheet(h.file, sheet)
	if err != nil {
		return Dimension{}, err
	}
	return measure(h.file, name)
}

// ValidateSheetName 校验工作表名是否符合 Excel 规则。
func ValidateSheetName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 31 {
		return false
	}
	for _, ch := range []string{`\`, "/", "?", "*", "[", "]", ":"} {
		if strings.Contains(name, ch) {
			return false
		}
	}
	return true
}
