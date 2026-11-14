package excel

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/xuri/excelize/v2"
)

// randomSuffix 生成一小段随机串，用于临时文件名，避免并发冲突。
func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "tmp"
	}
	return hex.EncodeToString(b[:])
}

// rowCell 把行号转成该行第一列的单元格名（StreamWriter.SetRow 需要单元格地址）。
func rowCell(row int) string {
	name, _ := excelize.CoordinatesToCellName(1, row)
	return name
}
