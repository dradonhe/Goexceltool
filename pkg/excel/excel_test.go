package excel

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseRowRange(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want []int
	}{
		{"1", 10, []int{1}},
		{"1,3,5", 10, []int{1, 3, 5}},
		{"1-3", 10, []int{1, 2, 3}},
		{"3-1", 10, []int{1, 2, 3}},        // 反向写法自动纠正
		{"1，3", 10, []int{1, 3}},           // 全角逗号
		{"1、3", 10, []int{1, 3}},           // 顿号
		{" 1 , 2 ", 10, []int{1, 2}},       //
		{"8-20", 10, []int{8, 9, 10}},      // 越界部分忽略
		{"0,1", 10, []int{1}},              // 0 无效
		{"", 10, nil},                      //
		{"abc", 10, nil},                   //
		{"1-3,2-4", 10, []int{1, 2, 3, 4}}, // 去重
	}
	for _, c := range cases {
		got := ParseRowRange(c.in, c.max)
		if len(got) != len(c.want) {
			t.Fatalf("ParseRowRange(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("ParseRowRange(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestParseColumnRange(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want []int
	}{
		{"A", 10, []int{1}},
		{"a,c", 10, []int{1, 3}},
		{"A,C,E-G", 10, []int{1, 3, 5, 6, 7}},
		{"1,3-4", 10, []int{1, 3, 4}}, // 也接受数字
	}
	for _, c := range cases {
		got := ParseColumnRange(c.in, c.max)
		if len(got) != len(c.want) {
			t.Fatalf("ParseColumnRange(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("ParseColumnRange(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

// newStyledWorkbook 造一个带样式、合并单元格和公式的文件，用来验证「删除后不丢格式」。
func newStyledWorkbook(t *testing.T) (path string, boldStyle int) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "src.xlsx")

	f := excelize.NewFile()
	defer f.Close()

	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		t.Fatalf("NewStyle: %v", err)
	}

	f.SetCellValue("Sheet1", "A1", "标题")
	f.SetCellValue("Sheet1", "A2", "数据1")
	f.SetCellValue("Sheet1", "B2", "数据2")
	f.SetCellValue("Sheet1", "A3", "数据3")
	f.SetCellValue("Sheet1", "B3", "=1+1")
	f.SetCellFormula("Sheet1", "B3", "=1+1")
	if err := f.SetCellStyle("Sheet1", "A2", "A2", bold); err != nil {
		t.Fatalf("SetCellStyle: %v", err)
	}
	if err := f.MergeCell("Sheet1", "A2", "B2"); err != nil {
		t.Fatalf("MergeCell: %v", err)
	}
	if err := f.SetColWidth("Sheet1", "A", "A", 30); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	return path, bold
}

func TestApplyDeleteKeepsFormatting(t *testing.T) {
	src, bold := newStyledWorkbook(t)
	dst := filepath.Join(t.TempDir(), "out.xlsx")

	report, err := ApplyDelete(src, dst, DeleteOptions{FirstRows: 1})
	if err != nil {
		t.Fatalf("ApplyDelete: %v", err)
	}
	if report.Before.Rows != 3 || report.After.Rows != 2 {
		t.Fatalf("行列数不符合预期: %+v", report)
	}

	f, err := excelize.OpenFile(dst)
	if err != nil {
		t.Fatalf("打开输出失败: %v", err)
	}
	defer f.Close()

	// 原来的第 2 行（带粗体、合并、公式）应该上移到第 1 行
	style, err := f.GetCellStyle("Sheet1", "A1")
	if err != nil {
		t.Fatalf("GetCellStyle: %v", err)
	}
	if style != bold {
		t.Fatalf("样式丢失：got style %d, want %d", style, bold)
	}

	merges, err := f.GetMergeCells("Sheet1")
	if err != nil {
		t.Fatalf("GetMergeCells: %v", err)
	}
	if len(merges) != 1 || merges[0].GetStartAxis() != "A1" || merges[0].GetEndAxis() != "B1" {
		t.Fatalf("合并单元格没有跟着上移: %+v", merges)
	}

	formula, err := f.GetCellFormula("Sheet1", "B2")
	if err != nil {
		t.Fatalf("GetCellFormula: %v", err)
	}
	if formula != "1+1" {
		t.Fatalf("公式丢失: %q", formula)
	}

	width, err := f.GetColWidth("Sheet1", "A")
	if err != nil {
		t.Fatalf("GetColWidth: %v", err)
	}
	if width != 30 {
		t.Fatalf("列宽丢失: %v", width)
	}
}

func TestApplyDeleteColumnsAndRanges(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.xlsx")
	f := excelize.NewFile()
	for c, v := range []string{"A", "B", "C", "D"} {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		f.SetCellValue("Sheet1", cell, v)
	}
	if err := f.SaveAs(src); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	f.Close()

	dst := filepath.Join(dir, "out.xlsx")
	report, err := ApplyDelete(src, dst, DeleteOptions{FirstCols: 1, CustomCols: "D"})
	if err != nil {
		t.Fatalf("ApplyDelete: %v", err)
	}
	if len(report.DeletedCols) != 2 {
		t.Fatalf("应删除 2 列，实际 %v", report.DeletedCols)
	}

	out, err := excelize.OpenFile(dst)
	if err != nil {
		t.Fatalf("打开输出失败: %v", err)
	}
	defer out.Close()
	rows, err := out.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != 2 || rows[0][0] != "B" || rows[0][1] != "C" {
		t.Fatalf("删除列结果错误: %v", rows)
	}
}

func TestApplyDeleteRejectsWholeSheet(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "x")
	if err := f.SaveAs(src); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	f.Close()

	if _, err := ApplyDelete(src, filepath.Join(t.TempDir(), "o.xlsx"), DeleteOptions{FirstRows: 5, FirstCols: 5}); err == nil {
		t.Fatal("删除整表应该被拒绝")
	}
}

func TestMergeFilesAlignsByHeader(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, rows [][]interface{}) string {
		p := filepath.Join(dir, name)
		f := excelize.NewFile()
		for r, row := range rows {
			for c, v := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
				f.SetCellValue("Sheet1", cell, v)
			}
		}
		if err := f.SaveAs(p); err != nil {
			t.Fatalf("SaveAs: %v", err)
		}
		f.Close()
		return p
	}

	main := write("main.xlsx", [][]interface{}{
		{"ID", "姓名", "年龄"},
		{"1", "张三", "30"},
	})
	input := write("input.xlsx", [][]interface{}{
		{"姓名", "ID"},
		{"李四", "2"},
	})

	out := filepath.Join(dir, "out.xlsx")
	report, err := MergeFiles(main, []string{input}, "Sheet1", out)
	if err != nil {
		t.Fatalf("MergeFiles: %v", err)
	}
	if report.Rows != 2 || report.Files != 2 {
		t.Fatalf("合并统计错误: %+v", report)
	}

	f, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatalf("打开输出失败: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	// 行数 3；末列是空串时 excelize 不会物化该单元格，所以按前两列断言
	if len(rows) != 3 {
		t.Fatalf("行数应为 3，实际 %d: %v", len(rows), rows)
	}
	if len(rows[2]) < 2 || rows[2][0] != "2" || rows[2][1] != "李四" {
		t.Fatalf("对齐结果错误: %v", rows[2])
	}
	if len(rows[2]) > 2 {
		if rows[2][2] != "" {
			t.Fatalf("缺失列应留空: %v", rows[2])
		}
	}
}

func writeJoinFixture(t *testing.T, dir, name string, rows [][]interface{}) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f := excelize.NewFile()
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellValue("Sheet1", cell, v); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
		}
	}
	if err := f.SaveAs(p); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	f.Close()
	return p
}

func TestRunJoinTypes(t *testing.T) {
	dir := t.TempDir()

	left := writeJoinFixture(t, dir, "left.xlsx", [][]interface{}{
		{"ID", "姓名"},
		{"1", "张三"},
		{"2", "李四"},
		{"3", "王五"},
	})
	right := writeJoinFixture(t, dir, "right.xlsx", [][]interface{}{
		{"ID", "部门"},
		{"2", "销售部"},
		{"3", "技术部"},
		{"4", "财务部"},
	})

	run := func(joinType string) (int, []string) {
		out := filepath.Join(dir, "out_"+joinType+".xlsx")
		report, err := RunJoin(JoinSpec{
			Base: JoinTable{File: left, Sheet: "Sheet1"},
			Rules: []JoinRule{{
				LeftKey:  "ID",
				Type:     joinType,
				Right:    JoinTable{File: right, Sheet: "Sheet1"},
				RightKey: "ID",
			}},
		}, out)
		if err != nil {
			t.Fatalf("%s 连接失败: %v", joinType, err)
		}
		f, err := excelize.OpenFile(out)
		if err != nil {
			t.Fatalf("打开 %s 输出失败: %v", joinType, err)
		}
		defer f.Close()
		rows, err := f.GetRows("Sheet1")
		if err != nil {
			t.Fatalf("GetRows: %v", err)
		}
		return report.Rows, rows[0]
	}

	if n, _ := run("inner"); n != 2 {
		t.Fatalf("INNER 应为 2 行，实际 %d", n)
	}
	if n, _ := run("left"); n != 3 {
		t.Fatalf("LEFT 应为 3 行，实际 %d", n)
	}
	if n, _ := run("right"); n != 3 {
		t.Fatalf("RIGHT 应为 3 行（2 条匹配 + 1 条右表独有），实际 %d", n)
	}
	if n, _ := run("notin"); n != 1 {
		t.Fatalf("NOT IN 应为 1 行，实际 %d", n)
	}

	// 输出字段筛选
	out := filepath.Join(dir, "out_fields.xlsx")
	report, err := RunJoin(JoinSpec{
		Base:         JoinTable{File: left, Sheet: "Sheet1"},
		OutputFields: "table1.姓名,table2.部门",
		Rules: []JoinRule{{
			LeftKey: "ID", Type: "inner",
			Right: JoinTable{File: right, Sheet: "Sheet1"}, RightKey: "ID",
		}},
	}, out)
	if err != nil {
		t.Fatalf("字段筛选失败: %v", err)
	}
	if report.Cols != 2 || report.Header[0] != "姓名" || report.Header[1] != "部门" {
		t.Fatalf("字段筛选结果错误: %+v", report)
	}
}

func TestRunJoinChained(t *testing.T) {
	dir := t.TempDir()
	a := writeJoinFixture(t, dir, "a.xlsx", [][]interface{}{
		{"ID", "姓名"}, {"1", "张三"}, {"2", "李四"},
	})
	b := writeJoinFixture(t, dir, "b.xlsx", [][]interface{}{
		{"ID", "部门"}, {"1", "技术部"}, {"2", "销售部"},
	})
	c := writeJoinFixture(t, dir, "c.xlsx", [][]interface{}{
		{"部门", "经理"}, {"技术部", "老王"}, {"销售部", "老李"},
	})

	out := filepath.Join(dir, "chain.xlsx")
	report, err := RunJoin(JoinSpec{
		Base: JoinTable{File: a, Sheet: "Sheet1"},
		Rules: []JoinRule{
			{LeftKey: "ID", Type: "left", Right: JoinTable{File: b, Sheet: "Sheet1"}, RightKey: "ID"},
			{LeftKey: "部门", Type: "left", Right: JoinTable{File: c, Sheet: "Sheet1"}, RightKey: "部门"},
		},
	}, out)
	if err != nil {
		t.Fatalf("链式连接失败: %v", err)
	}
	if report.Rows != 2 || report.Cols != 6 {
		t.Fatalf("链式连接结果错误: %+v", report)
	}
}
