package excel

import (
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// 范围解析约定（全项目统一）：
//   - 行号、列号一律 1 基（和第 1 行 / A 列对应）
//   - 内部转换成 0 基只允许发生在一处（见 delete.go）
//
// 支持的写法（尽量宽容，避免用户因为全角符号/空格被判错）：
//   行："1,3,5-10"、"5-3"（自动纠正）、"1 3 5"、"1，3"
//   列："A,C,E-G"、"e-g"、"1,3-5"（也接受数字）

// rangeReplacer 统一常见的全角/中文标点。
var rangeReplacer = strings.NewReplacer(
	"，", ",", "、", ",", "；", ",", ";", ",",
	"～", "-", "—", "-", "–", "-", "－", "-",
	"\u3000", " ", "\t", " ", "\n", " ", "\r", " ",
)

// ParseRowRange 解析行范围，返回升序去重的 1 基行号，超出 [1, maxRows] 的忽略。
func ParseRowRange(rangeStr string, maxRows int) []int {
	return parseRange(rangeStr, maxRows, func(token string) (int, bool) {
		n, err := strconv.Atoi(token)
		if err != nil {
			return 0, false
		}
		return n, true
	})
}

// ParseColumnRange 解析列范围，返回升序去重的 1 基列号，超出 [1, maxCols] 的忽略。
// 既接受 "A,C,E-G"，也接受 "1,3-5"。
func ParseColumnRange(rangeStr string, maxCols int) []int {
	return parseRange(rangeStr, maxCols, func(token string) (int, bool) {
		if n, err := strconv.Atoi(token); err == nil {
			return n, true
		}
		n, err := excelize.ColumnNameToNumber(token)
		if err != nil || n < 1 {
			return 0, false
		}
		return n, true
	})
}

// parseRange 是行/列范围解析的公共实现，parse 负责把单个 token 转成 1 基序号。
func parseRange(rangeStr string, max int, parse func(string) (int, bool)) []int {
	if strings.TrimSpace(rangeStr) == "" || max < 1 {
		return nil
	}

	var out []int
	for _, part := range strings.Split(rangeReplacer.Replace(rangeStr), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 单值
		if !strings.Contains(part, "-") {
			if n, ok := parse(part); ok && n >= 1 && n <= max {
				out = append(out, n)
			}
			continue
		}

		// 区间，如 3-5 / E-G
		bounds := strings.Split(part, "-")
		if len(bounds) != 2 {
			continue
		}
		start, ok1 := parse(strings.TrimSpace(bounds[0]))
		end, ok2 := parse(strings.TrimSpace(bounds[1]))
		if !ok1 || !ok2 {
			continue
		}
		if start > end { // 容忍 5-3 这种反向写法
			start, end = end, start
		}
		if start < 1 {
			start = 1
		}
		if end > max {
			end = max
		}
		for i := start; i <= end; i++ {
			out = append(out, i)
		}
	}
	return SortUnique(out)
}

// SortUnique 升序去重。
func SortUnique(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// firstN 返回 1..n。
func firstN(n, max int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n && i <= max; i++ {
		out = append(out, i)
	}
	return out
}

// lastN 返回末尾 n 个序号（升序）。
func lastN(n, max int) []int {
	if n <= 0 {
		return nil
	}
	start := max - n + 1
	if start < 1 {
		start = 1
	}
	out := make([]int, 0, max-start+1)
	for i := start; i <= max; i++ {
		out = append(out, i)
	}
	return out
}
