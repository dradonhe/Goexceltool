package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/file/abc.xlsx", "abc.xlsx"},          // 历史 bug：曾变成 "file abc.xlsx"
		{`C:\Users\me\报表.xlsx`, "报表.xlsx"},      // 浏览器可能带上完整路径
		{`/data/2024/员工 名单.xlsx`, "员工 名单.xlsx"}, // 中间的空格必须保留
		{"  spaced.xlsx  ", "spaced.xlsx"},      // 首尾空白去掉
		{"a<b>c|d?.xlsx", "a_b_c_d_.xlsx"},      // 非法字符替换
		{"", "result.xlsx"},                     //
		{"CON", "_CON.xlsx"},                    // Windows 保留名
		{"无扩展名", "无扩展名.xlsx"},                   //
	}
	for _, c := range cases {
		if got := SanitizeFileName(c.in); got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOutputFileName(t *testing.T) {
	cases := []struct {
		src, mode, suffix, want string
	}{
		{"a.xlsx", "overwrite", "", "a.xlsx"},
		{"a.xlsx", "newfile", "_processed", "a_processed.xlsx"},
		{"a.xlsx", "newfile", "", "a_processed.xlsx"},
		{"a.xlsx", "newfile", "/bad\\name", "a_bad_name.xlsx"},
		{"a.XLSX", "overwrite", "", "a.XLSX"},
	}
	for _, c := range cases {
		if got := OutputFileName(c.src, c.mode, c.suffix); got != c.want {
			t.Errorf("OutputFileName(%q,%q,%q) = %q, want %q", c.src, c.mode, c.suffix, got, c.want)
		}
	}
}

func TestResolveDirJoining(t *testing.T) {
	dir := t.TempDir()

	got, err := OutputPolicy{Dir: dir, Mode: "newfile", Suffix: "_out"}.Resolve("/upload/abc.xlsx")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(dir, "abc_out.xlsx")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("输出路径必须是绝对路径: %q", got)
	}

	// 结尾带分隔符也应正常
	got2, err := OutputPolicy{Dir: dir + string(os.PathSeparator), Mode: "overwrite"}.Resolve("a.xlsx")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got2 != filepath.Join(dir, "a.xlsx") {
		t.Fatalf("got %q", got2)
	}
}

func TestResolveExactFilePath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "结果.xlsx")

	got, err := OutputPolicy{Dir: target}.Resolve("whatever.xlsx")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != target {
		t.Fatalf("got %q, want %q", got, target)
	}

	// 没写扩展名时按目录处理：在里面生成 a.xlsx
	sub := filepath.Join(dir, "结果2")
	got2, err := OutputPolicy{Dir: sub}.Resolve("a.xlsx")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got2 != filepath.Join(sub, "a.xlsx") {
		t.Fatalf("got %q", got2)
	}
}

func TestResolveBatchRejectsFileTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.xlsx")
	if _, err := (OutputPolicy{Dir: target, Batch: true}).Resolve("a.xlsx"); err == nil {
		t.Fatal("批量模式下把输出路径当文件应该报错")
	}
}

func TestResolveName(t *testing.T) {
	dir := t.TempDir()
	got, err := OutputPolicy{Dir: dir, Name: "合并结果"}.Resolve("ignored.xlsx")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != filepath.Join(dir, "合并结果.xlsx") {
		t.Fatalf("got %q", got)
	}
}

func TestUniquePath(t *testing.T) {
	used := map[string]bool{}
	a := UniquePath("/x/a.xlsx", used)
	b := UniquePath("/x/a.xlsx", used)
	c := UniquePath("/x/a.xlsx", used)
	if a != "/x/a.xlsx" || b != "/x/a_2.xlsx" || c != "/x/a_3.xlsx" {
		t.Fatalf("去重失败: %q %q %q", a, b, c)
	}
}

func TestIsSubPath(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a", "b.xlsx")
	if !IsSubPath(root, inside) {
		t.Fatal("应判定为子路径")
	}
	if IsSubPath(root, filepath.Join(root, "..", "escape.xlsx")) {
		t.Fatal("不应判定为子路径")
	}
}

func TestCleanUserPath(t *testing.T) {
	if got := CleanUserPath("  /data/out  "); got != filepath.Clean("/data/out") {
		t.Fatalf("got %q", got)
	}
	if got := CleanUserPath(`" /data/out/ "`); got != filepath.Clean("/data/out") {
		t.Fatalf("got %q", got)
	}
}
