package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// 本文件集中处理「输出路径」相关的所有换算。
//
// 历史上出过的问题：
//   - 用户填 /data/out，结果生成的文件名变成 "/data/out abc.xlsx"（目录和文件名被空格拼在一起）
//   - 上传文件名里带了浏览器给的完整路径（如 C:\x\a.xlsx 或 /file/abc.xlsx），
//     直接拿来做 filepath.Join 会拼出 "/file abc.xlsx" 之类的怪路径
//   - 覆盖原文件时一边读一边写，程序崩了就剩半个文件
//
// 处理原则：
//   1. 目录和文件名永远用 filepath.Join 拼接，绝不用空格/字符串相加
//   2. 文件名先用 SanitizeFileName 去掉路径部分和非法字符
//   3. 用户既可能填目录，也可能直接填一个 .xlsx 全路径，两种都要认
//   4. 最终路径一定是绝对路径，并原样回显给用户

// OutputPolicy 描述一次输出策略。
type OutputPolicy struct {
	Dir    string `json:"dir"`    // 用户填的输出目录，也可直接填完整文件路径
	Mode   string `json:"mode"`   // overwrite=沿用原文件名；newfile=原名+后缀
	Suffix string `json:"suffix"` // newfile 模式的后缀，如 "_processed"
	Name   string `json:"name"`   // 合并/连接时直接指定文件名
	Batch  bool   `json:"batch"`  // 多文件批量：不允许把 Dir 当成具体文件
}

const xlsxExt = ".xlsx"

// DefaultOutputDir 返回默认输出目录：可执行文件旁的 output/。
// 取不到可执行文件路径时退回当前工作目录。
func DefaultOutputDir() string {
	exe, err := os.Executable()
	if err != nil {
		if wd, err := os.Getwd(); err == nil {
			return filepath.Join(wd, "output")
		}
		return "output"
	}
	return filepath.Join(filepath.Dir(exe), "output")
}

// SanitizeFileName 只保留文件名本身。
//
// 关键场景：浏览器上传的 Filename 可能是 "C:\\Users\\a\\测试 表.xlsx" 或
// "/file/abc.xlsx"；这里同时按 "/" 和 "\\" 切分，取最后一段，避免把目录拼进文件名。
func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r == 0, r < 0x20, r == 0x7f:
			return -1
		case strings.ContainsRune(`<>:"|?*`, r):
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		name = "result" + xlsxExt
	}
	if isWindowsReserved(name) {
		name = "_" + name
	}
	if !strings.EqualFold(filepath.Ext(name), xlsxExt) && filepath.Ext(name) == "" {
		name += xlsxExt
	}
	return name
}

// isWindowsReserved 处理 CON/PRN/AUX/NUL/COM1.. 这类保留名。
func isWindowsReserved(name string) bool {
	base := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

// LookLikeFilePath 判断用户输入是不是一个具体文件（而不是目录）。
func LookLikeFilePath(p string) bool {
	if p == "" {
		return false
	}
	// 以分隔符结尾的一定是目录
	if strings.HasSuffix(p, "/") || strings.HasSuffix(p, `\`) || strings.HasSuffix(p, string(os.PathSeparator)) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext == "" {
		return false
	}
	switch ext {
	case xlsxExt, ".xls", ".xlsm", ".xlsb", ".csv":
		return true
	}
	// 已有同名文件且不是目录 → 当文件处理
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// CleanUserPath 清理用户输入：去空白、去引号、展开 ~、统一分隔符。
func CleanUserPath(p string) string {
	// 反复去掉成对的空白和引号：用户可能写成 "  /data/out  " 这种形式
	for {
		trimmed := strings.TrimSpace(p)
		trimmed = strings.Trim(trimmed, `"'`)
		if trimmed == p {
			break
		}
		p = trimmed
	}
	p = strings.Map(func(r rune) rune {
		if r == 0 || unicode.IsControl(r) {
			return -1
		}
		return r
	}, p)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), string(os.PathSeparator)))
		}
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// Resolve 计算最终输出文件的绝对路径。
//
// srcName 是输入文件的原始文件名（批量模式下逐个传入）。
func (p OutputPolicy) Resolve(srcName string) (string, error) {
	srcName = SanitizeFileName(srcName)

	dir := CleanUserPath(p.Dir)
	if dir == "" {
		dir = DefaultOutputDir()
	}

	// 用户直接填了完整文件路径（单文件场景）
	if LookLikeFilePath(dir) {
		if p.Batch {
			return "", fmt.Errorf("批量处理时输出路径必须是目录，不能是文件：%q", dir)
		}
		if p.Name == "" {
			if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
				return "", fmt.Errorf("创建输出目录失败: %w", err)
			}
			out := ensureXLSX(dir)
			return absPath(out)
		}
	}

	// 判断是目录：不存在就按目录创建
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		return "", fmt.Errorf("输出路径 %q 是一个文件，批量处理时需要指定目录", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	name := p.Name
	if name != "" {
		name = ensureXLSX(SanitizeFileName(name))
	} else {
		name = OutputFileName(srcName, p.Mode, p.Suffix)
	}
	return absPath(filepath.Join(dir, name))
}

// OutputFileName 生成输出文件名（不含目录）。
//   - overwrite: 沿用原文件名
//   - newfile:   原名 + 后缀
func OutputFileName(srcName, mode, suffix string) string {
	base := strings.TrimSuffix(srcName, filepath.Ext(srcName))
	ext := filepath.Ext(srcName)
	if ext == "" || !strings.EqualFold(ext, xlsxExt) {
		ext = xlsxExt
	}

	if strings.EqualFold(mode, "newfile") {
		suffix = sanitizeSuffix(suffix)
		if suffix == "" {
			suffix = "_processed"
		}
		return base + suffix + ext
	}
	return base + ext
}

// sanitizeSuffix 去掉后缀里的路径分隔符等危险字符。
func sanitizeSuffix(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == 0, r < 0x20, r == 0x7f:
			return -1
		case strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		}
		return r
	}, s)
	return strings.Trim(s, " .")
}

// ensureXLSX 保证结尾是 .xlsx。
func ensureXLSX(p string) string {
	if strings.EqualFold(filepath.Ext(p), xlsxExt) {
		return p
	}
	return strings.TrimSuffix(p, filepath.Ext(p)) + xlsxExt
}

func absPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("解析路径失败: %w", err)
	}
	return abs, nil
}

// UniquePath 在同一批次里避免重名：a.xlsx → a_2.xlsx → a_3.xlsx。
func UniquePath(p string, used map[string]bool) string {
	if !used[strings.ToLower(p)] {
		used[strings.ToLower(p)] = true
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if !used[strings.ToLower(candidate)] {
			used[strings.ToLower(candidate)] = true
			return candidate
		}
	}
	return p
}

// IsSubPath 判断 child 是否位于 root 之内（用于下载白名单）。
func IsSubPath(root, child string) bool {
	if root == "" || child == "" {
		return false
	}
	r, err1 := filepath.Abs(root)
	c, err2 := filepath.Abs(child)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(r, c)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}
