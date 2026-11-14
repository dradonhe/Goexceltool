package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"exceltool/pkg/excel"
	"exceltool/web"
)

// Server 是本地 Web 服务：只监听回环地址，静态资源来自内嵌 FS。
type Server struct {
	version   string
	outputDir string

	mu    sync.RWMutex
	roots map[string]bool // 允许下载/打开的目录白名单
}

// Options 服务参数。
type Options struct {
	Version   string
	OutputDir string
}

// New 创建服务。
func New(opt Options) *Server {
	dir := opt.OutputDir
	if dir == "" {
		dir = DefaultOutputDir()
	}
	s := &Server{
		version:   opt.Version,
		outputDir: dir,
		roots:     map[string]bool{},
	}
	s.allowDir(dir)
	return s
}

// OutputDir 返回默认输出目录。
func (s *Server) OutputDir() string { return s.outputDir }

// Handler 返回路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", noCache(http.FileServer(http.FS(web.FS()))))

	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/sheet-info", s.handleSheetInfo)
	mux.HandleFunc("/api/delete-rows", s.handleDelete)
	mux.HandleFunc("/api/merge", s.handleMerge)
	mux.HandleFunc("/api/join", s.handleJoin)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/reveal", s.handleReveal)
	mux.HandleFunc("/api/exit", s.handleExit)

	return logRequests(mux)
}

// ---------- 通用 ----------

type resp struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusOK, resp{Status: "error", Message: err.Error()})
}

// ok 把业务字段合并进 {"status":"ok"} 一起返回。
func ok(w http.ResponseWriter, fields map[string]any) {
	out := make(map[string]any, len(fields)+1)
	out["status"] = "ok"
	for k, v := range fields {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// localOrigin 只允许本机页面调用敏感接口，避免任意网页驱动本程序。
func localOrigin(r *http.Request) bool {
	candidates := []string{r.Header.Get("Origin")}
	if ref := r.Header.Get("Referer"); ref != "" {
		candidates = append(candidates, ref)
	}
	for _, raw := range candidates {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			return false
		}
		switch u.Hostname() {
		case "127.0.0.1", "localhost", "::1":
			return true
		}
		return false
	}
	// 没有 Origin/Referer 视为本机直接访问（curl 等）
	return true
}

// ---------- 上传 ----------

type upload struct {
	Path string // 临时文件路径
	Name string // 清洗后的原始文件名
}

// readUploads 把请求里的文件存到独立临时目录，调用方负责调用返回的 cleanup。
func (s *Server) readUploads(w http.ResponseWriter, r *http.Request, field string, limit int64) ([]upload, func(), error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, func() {}, fmt.Errorf("解析上传内容失败: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "exceltool-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	files := r.MultipartForm.File[field]
	if len(files) == 0 {
		cleanup()
		return nil, func() {}, fmt.Errorf("没有收到任何文件")
	}
	if len(files) > 200 {
		cleanup()
		return nil, func() {}, fmt.Errorf("一次最多处理 200 个文件")
	}

	used := map[string]bool{}
	out := make([]upload, 0, len(files))
	for _, fh := range files {
		name := UniquePath(SanitizeFileName(fh.Filename), used)
		dst := filepath.Join(tmpDir, name)
		if err := saveMultipart(fh, dst); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("保存上传文件 %s 失败: %w", fh.Filename, err)
		}
		out = append(out, upload{Path: dst, Name: name})
	}
	return out, cleanup, nil
}

func saveMultipart(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func formBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func formInt(v string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(v))
	return n
}

// ---------- 接口 ----------

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{
		"defaultOutputDir": s.outputDir,
		"os":               runtime.GOOS,
		"arch":             runtime.GOARCH,
		"version":          s.version,
	})
}

func (s *Server) handleSheetInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, resp{Status: "error", Message: "只支持 POST"})
		return
	}
	files, cleanup, err := s.readUploads(w, r, "file", 256<<20)
	if err != nil {
		fail(w, err)
		return
	}
	defer cleanup()

	up := files[0]
	sheets, err := excel.SheetNames(up.Path)
	if err != nil {
		fail(w, err)
		return
	}
	sheet := r.FormValue("sheet")
	if sheet == "" && len(sheets) > 0 {
		sheet = sheets[0]
	}
	dim, header, err := excel.SheetPreview(up.Path, sheet)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]any{
		"file":   up.Name,
		"sheets": sheets,
		"sheet":  sheet,
		"rows":   dim.Rows,
		"cols":   dim.Cols,
		"header": header,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, resp{Status: "error", Message: "只支持 POST"})
		return
	}
	files, cleanup, err := s.readUploads(w, r, "files", 1<<30)
	if err != nil {
		fail(w, err)
		return
	}
	defer cleanup()

	opt := excel.DeleteOptions{
		Sheet:      r.FormValue("sheetName"),
		FirstRows:  formInt(r.FormValue("firstRowsCount")),
		LastRows:   formInt(r.FormValue("lastRowsCount")),
		CustomRows: r.FormValue("customRowsInput"),
		FirstCols:  formInt(r.FormValue("firstColsCount")),
		LastCols:   formInt(r.FormValue("lastColsCount")),
		CustomCols: r.FormValue("customColsInput"),
	}
	// 只有勾选的项才生效
	if !formBool(r.FormValue("deleteFirstRows")) {
		opt.FirstRows = 0
	}
	if !formBool(r.FormValue("deleteLastRows")) {
		opt.LastRows = 0
	}
	if !formBool(r.FormValue("deleteCustomRows")) {
		opt.CustomRows = ""
	}
	if !formBool(r.FormValue("deleteFirstCols")) {
		opt.FirstCols = 0
	}
	if !formBool(r.FormValue("deleteLastCols")) {
		opt.LastCols = 0
	}
	if !formBool(r.FormValue("deleteCustomCols")) {
		opt.CustomCols = ""
	}
	if opt.Noop() {
		fail(w, fmt.Errorf("请至少勾选一种删除操作"))
		return
	}

	policy := OutputPolicy{
		Dir:    r.FormValue("outputDir"),
		Mode:   r.FormValue("outputMode"),
		Suffix: r.FormValue("suffix"),
		Batch:  len(files) > 1,
	}
	if policy.Mode == "" {
		policy.Mode = "overwrite"
	}

	type fileResult struct {
		Input       string          `json:"input"`
		Output      string          `json:"output"`
		Before      excel.Dimension `json:"before"`
		After       excel.Dimension `json:"after"`
		DeletedRows []int           `json:"deletedRows"`
		DeletedCols []int           `json:"deletedCols"`
		Error       string          `json:"error,omitempty"`
	}

	results := make([]fileResult, 0, len(files))
	used := map[string]bool{}
	processed := 0
	var firstOutDir string

	for _, up := range files {
		dst, err := policy.Resolve(up.Name)
		if err != nil {
			results = append(results, fileResult{Input: up.Name, Error: err.Error()})
			continue
		}
		dst = UniquePath(dst, used)

		report, err := excel.ApplyDelete(up.Path, dst, opt)
		if err != nil {
			results = append(results, fileResult{Input: up.Name, Output: dst, Error: err.Error()})
			continue
		}
		s.allowFile(dst)
		if firstOutDir == "" {
			firstOutDir = filepath.Dir(dst)
		}
		processed++
		results = append(results, fileResult{
			Input: up.Name, Output: dst,
			Before: report.Before, After: report.After,
			DeletedRows: report.DeletedRows, DeletedCols: report.DeletedCols,
		})
	}

	ok(w, map[string]any{
		"processed": processed,
		"total":     len(files),
		"outputDir": firstOutDir,
		"files":     results,
	})
}

func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, resp{Status: "error", Message: "只支持 POST"})
		return
	}
	files, cleanup, err := s.readUploads(w, r, "files", 1<<30)
	if err != nil {
		fail(w, err)
		return
	}
	defer cleanup()

	if len(files) < 2 {
		fail(w, fmt.Errorf("至少需要两个文件才能合并"))
		return
	}

	mainIdx := formInt(r.FormValue("mainIndex"))
	if mainIdx < 0 || mainIdx >= len(files) {
		mainIdx = 0
	}

	policy := OutputPolicy{
		Dir:  r.FormValue("outputDir"),
		Name: r.FormValue("outputName"),
	}
	if policy.Name == "" {
		policy.Name = "merged_" + time.Now().Format("20060102_150405") + ".xlsx"
	}
	dst, err := policy.Resolve("merged.xlsx")
	if err != nil {
		fail(w, err)
		return
	}

	inputs := make([]string, 0, len(files)-1)
	for i, f := range files {
		if i == mainIdx {
			continue
		}
		inputs = append(inputs, f.Path)
	}

	report, err := excel.MergeFiles(files[mainIdx].Path, inputs, r.FormValue("sheetName"), dst)
	if err != nil {
		fail(w, err)
		return
	}
	s.allowFile(dst)

	ok(w, map[string]any{
		"output":    dst,
		"outputDir": filepath.Dir(dst),
		"report":    report,
	})
}

type joinRequest struct {
	BaseIndex    int    `json:"baseIndex"`
	SheetName    string `json:"sheetName"`
	OutputFields string `json:"outputFields"`
	OutputSheet  string `json:"outputSheet"`
	OutputDir    string `json:"outputDir"`
	OutputName   string `json:"outputName"`
	Rules        []struct {
		LeftKey    string `json:"leftKey"`
		Type       string `json:"type"`
		RightIndex int    `json:"rightIndex"`
		RightKey   string `json:"rightKey"`
		RightSheet string `json:"rightSheet"`
	} `json:"rules"`
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, resp{Status: "error", Message: "只支持 POST"})
		return
	}
	files, cleanup, err := s.readUploads(w, r, "files", 1<<30)
	if err != nil {
		fail(w, err)
		return
	}
	defer cleanup()

	var req joinRequest
	if err := json.Unmarshal([]byte(r.FormValue("spec")), &req); err != nil {
		fail(w, fmt.Errorf("连接参数解析失败: %w", err))
		return
	}
	if len(req.Rules) == 0 {
		fail(w, fmt.Errorf("至少需要一条连接规则"))
		return
	}

	idx := func(i int) (string, error) {
		if i < 0 || i >= len(files) {
			return "", fmt.Errorf("规则引用了不存在的文件序号 %d", i)
		}
		return files[i].Path, nil
	}

	basePath, err := idx(req.BaseIndex)
	if err != nil {
		fail(w, err)
		return
	}

	spec := excel.JoinSpec{
		Base:         excel.JoinTable{File: basePath, Sheet: req.SheetName},
		OutputFields: req.OutputFields,
		OutputSheet:  req.OutputSheet,
	}
	for i, rule := range req.Rules {
		rightPath, err := idx(rule.RightIndex)
		if err != nil {
			fail(w, fmt.Errorf("第 %d 条规则: %w", i+1, err))
			return
		}
		spec.Rules = append(spec.Rules, excel.JoinRule{
			LeftKey:  rule.LeftKey,
			Type:     rule.Type,
			Right:    excel.JoinTable{File: rightPath, Sheet: rule.RightSheet},
			RightKey: rule.RightKey,
		})
	}

	policy := OutputPolicy{Dir: req.OutputDir, Name: req.OutputName}
	if policy.Name == "" {
		policy.Name = "joined_" + time.Now().Format("20060102_150405") + ".xlsx"
	}
	dst, err := policy.Resolve("joined.xlsx")
	if err != nil {
		fail(w, err)
		return
	}

	report, err := excel.RunJoin(spec, dst)
	if err != nil {
		fail(w, err)
		return
	}
	s.allowFile(dst)

	ok(w, map[string]any{
		"output":    dst,
		"outputDir": filepath.Dir(dst),
		"report":    report,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	p := CleanUserPath(r.URL.Query().Get("path"))
	if p == "" {
		http.Error(w, "缺少 path 参数", http.StatusBadRequest)
		return
	}
	abs, err := filepath.Abs(p)
	if err != nil || !s.allowed(abs) {
		http.Error(w, "该路径不在可下载范围内", http.StatusForbidden)
		return
	}
	fi, err := os.Stat(abs)
	if err != nil || fi.IsDir() {
		http.Error(w, "文件不存在", http.StatusNotFound)
		return
	}
	name := filepath.Base(abs)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	http.ServeFile(w, r, abs)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, resp{Status: "error", Message: "只支持 POST"})
		return
	}
	if !localOrigin(r) {
		writeJSON(w, http.StatusForbidden, resp{Status: "error", Message: "只允许本机页面调用"})
		return
	}
	p := CleanUserPath(r.URL.Query().Get("path"))
	abs, err := filepath.Abs(p)
	if err != nil || !s.allowed(abs) {
		fail(w, fmt.Errorf("该路径不在允许范围内"))
		return
	}
	if err := reveal(abs); err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]any{"path": abs})
}

func (s *Server) handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, resp{Status: "error", Message: "只支持 POST"})
		return
	}
	if !localOrigin(r) {
		writeJSON(w, http.StatusForbidden, resp{Status: "error", Message: "只允许本机页面调用"})
		return
	}
	ok(w, map[string]any{"message": "程序即将退出"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
}

// ---------- 下载白名单 ----------

func (s *Server) allowDir(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.roots[abs] = true
	s.mu.Unlock()
}

func (s *Server) allowFile(p string) { s.allowDir(filepath.Dir(p)) }

func (s *Server) allowed(p string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for root := range s.roots {
		if IsSubPath(root, p) {
			return true
		}
	}
	return false
}

// ---------- 平台相关 ----------

// reveal 在系统文件管理器里打开路径（标准库实现，无 cgo）。
func reveal(p string) error {
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		p = filepath.Dir(p)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法打开文件管理器: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
