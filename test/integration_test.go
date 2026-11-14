package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"exceltool/internal/app"
)

// 端到端测试：走真实 HTTP 路由（httptest），覆盖删除 / 合并 / 连接和路径处理。

type client struct {
	t   *testing.T
	srv *httptest.Server
}

func newClient(t *testing.T) (*client, string) {
	t.Helper()
	outDir := t.TempDir()
	s := app.New(app.Options{Version: "test", OutputDir: outDir})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv}, outDir
}

func (c *client) post(path string, body *bytes.Buffer, contentType string) map[string]any {
	c.t.Helper()
	res, err := c.srv.Client().Post(c.srv.URL+path, contentType, body)
	if err != nil {
		c.t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		c.t.Fatalf("POST %s: HTTP %d", path, res.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		c.t.Fatalf("解析响应失败: %v", err)
	}
	if out["status"] != "ok" {
		c.t.Fatalf("接口返回失败: %v", out["message"])
	}
	return out
}

// buildForm 组装 multipart，files 用 "字段名:磁盘路径:客户端文件名" 描述。
func buildForm(t *testing.T, fields map[string]string, files [][3]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("写字段失败: %v", err)
		}
	}
	for _, f := range files {
		field, diskPath, uploadName := f[0], f[1], f[2]
		part, err := w.CreateFormFile(field, uploadName)
		if err != nil {
			t.Fatalf("创建表单文件失败: %v", err)
		}
		data, err := os.ReadFile(diskPath)
		if err != nil {
			t.Fatalf("读取测试文件失败: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("写入表单文件失败: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭表单失败: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func testdata(name string) string {
	return filepath.Join("..", "testdata", name)
}

func mustExist(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("期望的文件不存在: %s (%v)", p, err)
	}
}

func dimensions(t *testing.T, p, sheet string) (int, int) {
	t.Helper()
	f, err := excelize.OpenFile(p)
	if err != nil {
		t.Fatalf("打开 %s 失败: %v", p, err)
	}
	defer f.Close()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", p, err)
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	return len(rows), cols
}

func TestDeleteEndpoint(t *testing.T) {
	c, outDir := newClient(t)

	body, ct := buildForm(t,
		map[string]string{
			"sheetName":       "Sheet1",
			"outputMode":      "newfile",
			"suffix":          "_out",
			"outputDir":       outDir,
			"deleteFirstRows": "true",
			"firstRowsCount":  "1",
			"deleteFirstCols": "true",
			"firstColsCount":  "1",
		},
		[][3]string{{"files", testdata("employee.xlsx"), "employee.xlsx"}},
	)

	out := c.post("/api/delete-rows", body, ct)
	files, _ := out["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("返回文件数不对: %v", out)
	}
	item := files[0].(map[string]any)
	if item["error"] != nil {
		t.Fatalf("处理失败: %v", item["error"])
	}

	// 关键断言：输出路径必须是 <outDir>/employee_out.xlsx，中间不能插空格
	want := filepath.Join(outDir, "employee_out.xlsx")
	if item["output"] != want {
		t.Fatalf("输出路径 = %q, want %q", item["output"], want)
	}
	mustExist(t, want)

	rows, cols := dimensions(t, want, "Sheet1")
	if rows != 8 || cols != 4 {
		t.Fatalf("删除后尺寸 = %d行×%d列, want 8×4", rows, cols)
	}
}

func TestDeleteSanitizesBogusUploadName(t *testing.T) {
	c, outDir := newClient(t)

	body, ct := buildForm(t,
		map[string]string{
			"sheetName":       "Sheet1",
			"outputMode":      "newfile",
			"suffix":          "_x",
			"outputDir":       outDir,
			"deleteFirstRows": "true",
			"firstRowsCount":  "1",
		},
		// 浏览器有时会上传带路径的文件名，这里模拟 /file/abc.xlsx
		[][3]string{{"files", testdata("employee.xlsx"), "/file/abc.xlsx"}},
	)

	out := c.post("/api/delete-rows", body, ct)
	item := out["files"].([]any)[0].(map[string]any)
	want := filepath.Join(outDir, "abc_x.xlsx")
	if item["output"] != want {
		t.Fatalf("输出路径 = %q, want %q", item["output"], want)
	}
	mustExist(t, want)
}

func TestDeleteExactFilePathWithSpaces(t *testing.T) {
	c, outDir := newClient(t)
	target := filepath.Join(outDir, "带 空格 的 目录", "结果 表.xlsx")

	body, ct := buildForm(t,
		map[string]string{
			"sheetName":       "Sheet1",
			"outputMode":      "overwrite",
			"outputDir":       target,
			"deleteFirstRows": "true",
			"firstRowsCount":  "1",
		},
		[][3]string{{"files", testdata("employee.xlsx"), "employee.xlsx"}},
	)

	out := c.post("/api/delete-rows", body, ct)
	item := out["files"].([]any)[0].(map[string]any)
	if item["output"] != target {
		t.Fatalf("输出路径 = %q, want %q", item["output"], target)
	}
	mustExist(t, target)
}

func TestDeleteTrailingSeparator(t *testing.T) {
	c, outDir := newClient(t)
	dir := filepath.Join(outDir, "子目录") + string(os.PathSeparator)

	body, ct := buildForm(t,
		map[string]string{
			"sheetName":       "Sheet1",
			"outputMode":      "overwrite",
			"outputDir":       dir,
			"deleteFirstRows": "true",
			"firstRowsCount":  "1",
		},
		[][3]string{{"files", testdata("department.xlsx"), "department.xlsx"}},
	)
	out := c.post("/api/delete-rows", body, ct)
	item := out["files"].([]any)[0].(map[string]any)
	want := filepath.Join(outDir, "子目录", "department.xlsx")
	if item["output"] != want {
		t.Fatalf("输出路径 = %q, want %q", item["output"], want)
	}
	mustExist(t, want)
}

func TestMergeEndpoint(t *testing.T) {
	c, outDir := newClient(t)

	body, ct := buildForm(t,
		map[string]string{
			"sheetName":  "Sheet1",
			"mainIndex":  "0",
			"outputDir":  outDir,
			"outputName": "合并 结果.xlsx",
		},
		[][3]string{
			{"files", testdata("employee.xlsx"), "employee.xlsx"},
			{"files", testdata("resigned.xlsx"), "resigned.xlsx"},
		},
	)

	out := c.post("/api/merge", body, ct)
	want := filepath.Join(outDir, "合并 结果.xlsx")
	if out["output"] != want {
		t.Fatalf("输出路径 = %q, want %q", out["output"], want)
	}
	mustExist(t, want)

	// employee 8 行 + resigned 3 行 + 表头 = 12 行
	rows, _ := dimensions(t, want, "Sheet1")
	if rows != 12 {
		t.Fatalf("合并后行数 = %d, want 12", rows)
	}
}

func TestJoinEndpointNotIn(t *testing.T) {
	c, outDir := newClient(t)

	body, ct := buildForm(t,
		map[string]string{
			"spec": fmt.Sprintf(`{"baseIndex":0,"sheetName":"Sheet1","outputDir":%q,"outputName":"notin.xlsx","rules":[{"leftKey":"员工ID","type":"notin","rightIndex":1,"rightKey":"员工ID","rightSheet":"Sheet1"}]}`, outDir),
		},
		[][3]string{
			{"files", testdata("employee.xlsx"), "employee.xlsx"},
			{"files", testdata("resigned.xlsx"), "resigned.xlsx"},
		},
	)

	out := c.post("/api/join", body, ct)
	want := filepath.Join(outDir, "notin.xlsx")
	if out["output"] != want {
		t.Fatalf("输出路径 = %q, want %q", out["output"], want)
	}
	mustExist(t, want)

	// 8 名员工 - 3 名离职 = 5 行 + 表头
	rows, _ := dimensions(t, want, "Sheet1")
	if rows != 6 {
		t.Fatalf("NOT IN 行数 = %d, want 6（含表头）", rows)
	}
}

func TestJoinEndpointInnerWithFields(t *testing.T) {
	c, outDir := newClient(t)

	spec := fmt.Sprintf(`{"baseIndex":0,"sheetName":"Sheet1","outputDir":%q,"outputName":"inner.xlsx","outputFields":"table1.姓名,table2.离职日期","rules":[{"leftKey":"员工ID","type":"inner","rightIndex":1,"rightKey":"员工ID","rightSheet":"Sheet1"}]}`, outDir)
	body, ct := buildForm(t, map[string]string{"spec": spec},
		[][3]string{
			{"files", testdata("employee.xlsx"), "employee.xlsx"},
			{"files", testdata("resigned.xlsx"), "resigned.xlsx"},
		},
	)

	out := c.post("/api/join", body, ct)
	report := out["report"].(map[string]any)
	if report["rows"].(float64) != 3 || report["cols"].(float64) != 2 {
		t.Fatalf("INNER 结果不对: %v", report)
	}
}

func TestDownloadRejectsOutsidePath(t *testing.T) {
	c, _ := newClient(t)

	res, err := c.srv.Client().Get(c.srv.URL + "/api/download?path=" + filepath.ToSlash(filepath.Join("..", "..", "etc", "passwd")))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("越权路径应被拒绝，实际 HTTP %d", res.StatusCode)
	}
}

func TestExitRequiresLocalOrigin(t *testing.T) {
	c, _ := newClient(t)

	req, _ := http.NewRequest(http.MethodPost, c.srv.URL+"/api/exit", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	res, err := c.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("外部 Origin 应被拒绝，实际 HTTP %d", res.StatusCode)
	}
}
