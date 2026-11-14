package excel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// JoinType 连接类型。
type JoinType string

const (
	JoinInner JoinType = "inner" // 内连接：只保留两边都匹配上的行
	JoinLeft  JoinType = "left"  // 左连接：保留左表全部行
	JoinRight JoinType = "right" // 右连接：保留右表全部行
	JoinNotIn JoinType = "notin" // 差集 A-B：左表键不在右表中的行
)

// ParseJoinType 兼容前端下拉框的各种写法。
func ParseJoinType(s string) (JoinType, error) {
	v := strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
	switch {
	case v == "", v == "INNER", v == "INNER JOIN", strings.Contains(v, "内连接"):
		return JoinInner, nil
	case v == "LEFT", v == "LEFT JOIN", strings.Contains(v, "左连接"):
		return JoinLeft, nil
	case v == "RIGHT", v == "RIGHT JOIN", strings.Contains(v, "右连接"):
		return JoinRight, nil
	case v == "NOT IN", v == "NOTIN", v == "A-B", v == "A NOT IN B", strings.Contains(v, "差集"):
		return JoinNotIn, nil
	}
	return "", fmt.Errorf("不支持的连接类型: %q（可用：INNER JOIN / LEFT JOIN / RIGHT JOIN / NOT IN）", s)
}

// JoinTable 参与连接的一张表。
type JoinTable struct {
	File  string `json:"file"`
	Sheet string `json:"sheet"`
}

// JoinRule 一条连接规则：左键（在已累积的结果里）↔ 右键（在右表里）。
type JoinRule struct {
	LeftKey  string    `json:"leftKey"`
	Type     string    `json:"type"`
	Right    JoinTable `json:"right"`
	RightKey string    `json:"rightKey"`
}

// JoinSpec 一次连接操作的完整描述。
// 规则按顺序执行：第 1 条的左键作用在基表上，之后每条作用在上一步的结果上。
type JoinSpec struct {
	Base         JoinTable  `json:"base"`
	Rules        []JoinRule `json:"rules"`
	OutputFields string     `json:"outputFields"`
	OutputSheet  string     `json:"outputSheet"`
}

// JoinReport 连接结果。
type JoinReport struct {
	Rows     int      `json:"rows"`
	Cols     int      `json:"cols"`
	Header   []string `json:"header"`
	Steps    []string `json:"steps"`
	Warnings []string `json:"warnings,omitempty"`
}

// table 内存中的二维表。
type table struct {
	header []string
	rows   [][]string
	origin []int // 每列来源步骤：0 = 基表，1 = 第 1 次连接的右表，以此类推
}

// RunJoin 执行连接并写出结果。
func RunJoin(spec JoinSpec, outputPath string) (*JoinReport, error) {
	if strings.TrimSpace(spec.Base.File) == "" {
		return nil, fmt.Errorf("请先选择基表")
	}
	if len(spec.Rules) == 0 {
		return nil, fmt.Errorf("至少需要一条连接规则")
	}

	cur, err := openTable(spec.Base.File, spec.Base.Sheet)
	if err != nil {
		return nil, fmt.Errorf("读取基表失败: %w", err)
	}

	report := &JoinReport{}
	for i, rule := range spec.Rules {
		jt, err := ParseJoinType(rule.Type)
		if err != nil {
			return nil, err
		}
		right, err := openTable(rule.Right.File, rule.Right.Sheet)
		if err != nil {
			return nil, fmt.Errorf("第 %d 条规则读取右表失败: %w", i+1, err)
		}

		next, warns, err := joinStep(cur, right, rule.LeftKey, rule.RightKey, jt, i+1)
		if err != nil {
			return nil, fmt.Errorf("第 %d 条规则失败: %w", i+1, err)
		}
		report.Warnings = append(report.Warnings, warns...)
		report.Steps = append(report.Steps, fmt.Sprintf("第 %d 步 %s：%s(%s) ⋈ %s(%s) → %d 行",
			i+1, jt, tableName(spec.Base.File), rule.LeftKey, tableName(rule.Right.File), rule.RightKey, len(next.rows)))
		cur = next
	}

	// 输出字段筛选
	idx, err := selectFields(cur, spec.OutputFields)
	if err != nil {
		return nil, err
	}

	outSheet := strings.TrimSpace(spec.OutputSheet)
	if outSheet == "" {
		outSheet = "Sheet1"
	}

	report.Rows = len(cur.rows)
	report.Cols = len(idx)
	report.Header = make([]string, len(idx))
	for i, c := range idx {
		report.Header[i] = cur.header[c]
	}

	if err := writeTable(outSheet, report.Header, cur.rows, idx, outputPath); err != nil {
		return nil, err
	}
	return report, nil
}

// openTable 读取整张表（第一行为表头）。
func openTable(path, sheet string) (*table, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开 %s 失败: %w", tableName(path), err)
	}
	defer f.Close()

	sheet, err = pickSheet(f, sheet)
	if err != nil {
		return nil, err
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("读取工作表 %q 失败: %w", sheet, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("工作表 %q 没有数据", sheet)
	}

	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}
	if width == 0 {
		return nil, fmt.Errorf("工作表 %q 的表头为空", sheet)
	}

	t := &table{rows: rows[1:], origin: make([]int, width)}
	// 补齐表头长度，避免后续按列名取值越界
	t.header = make([]string, width)
	for i := 0; i < width; i++ {
		if i < len(rows[0]) {
			t.header[i] = strings.TrimSpace(rows[0][i])
		}
	}
	return t, nil
}

// joinStep 执行一次连接。
func joinStep(left, right *table, leftKey, rightKey string, jt JoinType, step int) (*table, []string, error) {
	li, err := columnIndex(left.header, leftKey)
	if err != nil {
		return nil, nil, fmt.Errorf("左表找不到列 %q，可用列：%s", leftKey, strings.Join(nonEmpty(left.header), "、"))
	}
	ri, err := columnIndex(right.header, rightKey)
	if err != nil {
		return nil, nil, fmt.Errorf("右表找不到列 %q，可用列：%s", rightKey, strings.Join(nonEmpty(right.header), "、"))
	}

	// 右表建索引：键 → 行号
	index := make(map[string][]int, len(right.rows))
	for i, row := range right.rows {
		k := key(row, ri)
		if k == "" {
			continue
		}
		index[k] = append(index[k], i)
	}

	// 输出表头：左表 + 右表；notin 只保留左表
	out := &table{header: append([]string{}, left.header...), origin: append([]int{}, left.origin...)}
	var warns []string
	if jt != JoinNotIn {
		used := make(map[string]int, len(out.header))
		for _, h := range out.header {
			used[h]++
		}
		for i, h := range right.header {
			name := h
			if name == "" {
				name = fmt.Sprintf("列%d", i+1)
			}
			if used[name] > 0 {
				newName := fmt.Sprintf("%s_%d", name, step+1)
				warns = append(warns, fmt.Sprintf("右表列 %q 与已有列重名，已改名为 %q", name, newName))
				name = newName
			}
			used[name]++
			out.header = append(out.header, name)
			out.origin = append(out.origin, step)
		}
	}

	leftWidth := len(left.header)
	matchedRight := make(map[int]bool, len(right.rows))

	blankRight := make([]string, len(right.header))
	blankLeft := make([]string, leftWidth)

	for _, lrow := range left.rows {
		k := key(lrow, li)
		positions := index[k]
		if len(positions) == 0 {
			// 右表无匹配：只有左连接保留左表行
			if jt == JoinLeft {
				out.rows = append(out.rows, joinRows(lrow, blankRight, leftWidth))
			}
			if jt == JoinNotIn {
				out.rows = append(out.rows, pad(lrow, leftWidth))
			}
			continue
		}
		for _, pi := range positions {
			matchedRight[pi] = true
			if jt == JoinNotIn {
				break // 差集：只要有匹配就丢弃
			}
			out.rows = append(out.rows, joinRows(lrow, pad(right.rows[pi], len(right.header)), leftWidth))
		}
	}

	// 右连接：补齐右表独有行
	if jt == JoinRight {
		for i, rrow := range right.rows {
			if matchedRight[i] {
				continue
			}
			out.rows = append(out.rows, joinRows(blankLeft, pad(rrow, len(right.header)), leftWidth))
		}
	}

	return out, warns, nil
}

// selectFields 解析输出字段，返回要保留的列下标。
// 支持：留空（=全部）、"*"、"列名"、"tableN.列名"、"tableN.sheetX.*"、"表N.*"。
func selectFields(t *table, expr string) ([]int, error) {
	expr = strings.TrimSpace(rangeReplacer.Replace(expr))
	if expr == "" || expr == "*" {
		all := make([]int, len(t.header))
		for i := range all {
			all[i] = i
		}
		return all, nil
	}

	var picked []int
	seen := map[int]bool{}
	add := func(i int) {
		if !seen[i] {
			seen[i] = true
			picked = append(picked, i)
		}
	}

	for _, raw := range strings.Split(expr, ",") {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		if token == "*" {
			for i := range t.header {
				add(i)
			}
			continue
		}

		parts := strings.Split(token, ".")
		colName := strings.TrimSpace(parts[len(parts)-1])
		step := -1
		if len(parts) > 1 {
			step = parseTableRef(parts[0])
			if step < 0 {
				return nil, fmt.Errorf("输出字段 %q 的表名无法识别，应写成 table1.列名 或 列名", token)
			}
			step-- // table1 → 基表（origin 0）
		}

		if colName == "*" {
			found := false
			for i, o := range t.origin {
				if step < 0 || o == step {
					add(i)
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("输出字段 %q 没有匹配到任何列", token)
			}
			continue
		}

		found := false
		for i, h := range t.header {
			if h != colName {
				continue
			}
			if step >= 0 && t.origin[i] != step {
				continue
			}
			add(i)
			found = true
		}
		if !found {
			return nil, fmt.Errorf("输出字段 %q 在结果里不存在，可用列：%s", token, strings.Join(nonEmpty(t.header), "、"))
		}
	}

	if len(picked) == 0 {
		return nil, fmt.Errorf("输出字段为空")
	}
	return picked, nil
}

// parseTableRef 解析 table1 / 表1 / 1。
func parseTableRef(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "table")
	s = strings.TrimPrefix(s, "表")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return -1
	}
	return n
}

// writeTable 用 StreamWriter 写出结果。
func writeTable(sheet string, header []string, rows [][]string, keep []int, path string) error {
	out := excelize.NewFile()
	defer out.Close()

	created := false
	if sheet != "Sheet1" {
		if _, err := out.NewSheet(sheet); err != nil {
			return fmt.Errorf("创建输出工作表失败: %w", err)
		}
		created = true
	}
	sw, err := out.NewStreamWriter(sheet)
	if err != nil {
		return fmt.Errorf("初始化输出失败: %w", err)
	}

	head := make([]interface{}, len(header))
	for i, h := range header {
		head[i] = h
	}

	if err := sw.SetRow("A1", head); err != nil {
		return fmt.Errorf("写入表头失败: %w", err)
	}

	values := make([]interface{}, len(keep))
	for r, row := range rows {
		for i, c := range keep {
			if c < len(row) {
				values[i] = row[c]
			} else {
				values[i] = ""
			}
		}
		if err := sw.SetRow(rowCell(r+2), values); err != nil {
			return fmt.Errorf("写入第 %d 行失败: %w", r+2, err)
		}
	}
	if err := sw.Flush(); err != nil {
		return fmt.Errorf("写出结果失败: %w", err)
	}
	if created {
		if err := out.DeleteSheet("Sheet1"); err != nil {
			return fmt.Errorf("清理默认工作表失败: %w", err)
		}
	}
	out.SetActiveSheet(0)
	return saveAtomic(out, path)
}

// ---------- 小工具 ----------

func columnIndex(header []string, name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("连接键不能为空")
	}
	// 先精确匹配，再忽略大小写匹配
	for i, h := range header {
		if h == name {
			return i, nil
		}
	}
	for i, h := range header {
		if strings.EqualFold(h, name) {
			return i, nil
		}
	}
	// 最后尝试数字列号（1 基）
	if n, err := strconv.Atoi(name); err == nil && n >= 1 && n <= len(header) {
		return n - 1, nil
	}
	return 0, fmt.Errorf("找不到列 %q", name)
}

func key(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func pad(row []string, width int) []string {
	if len(row) >= width {
		return row
	}
	out := make([]string, width)
	copy(out, row)
	return out
}

func joinRows(left, right []string, leftWidth int) []string {
	out := make([]string, 0, leftWidth+len(right))
	l := pad(left, leftWidth)
	out = append(out, l...)
	out = append(out, right...)
	return out
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func tableName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}
