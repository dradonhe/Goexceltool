package excel

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestMergeSameHeader_Handler(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.xlsx")
	inputPath := filepath.Join(dir, "input.xlsx")
	outPath := filepath.Join(dir, "out.xlsx")
	sheet := "Sheet1"

	// create main file with header ID, Name, Age
	f := excelize.NewFile()
	f.DeleteSheet("Sheet1")
	f.NewSheet(sheet)
	f.SetCellValue(sheet, "A1", "ID")
	f.SetCellValue(sheet, "B1", "Name")
	f.SetCellValue(sheet, "C1", "Age")
	f.SetCellValue(sheet, "A2", "1")
	f.SetCellValue(sheet, "B2", "Alice")
	f.SetCellValue(sheet, "C2", "30")
	if err := f.SaveAs(mainPath); err != nil {
		t.Fatalf("failed to save main file: %v", err)
	}

	// create input file with header Name, ID (Age missing)
	g := excelize.NewFile()
	g.DeleteSheet("Sheet1")
	g.NewSheet(sheet)
	g.SetCellValue(sheet, "A1", "Name")
	g.SetCellValue(sheet, "B1", "ID")
	g.SetCellValue(sheet, "A2", "Bob")
	g.SetCellValue(sheet, "B2", "2")
	if err := g.SaveAs(inputPath); err != nil {
		t.Fatalf("failed to save input file: %v", err)
	}

	// call MergeSameHeader
	if err := MergeSameHeader(mainPath, []string{inputPath}, sheet, outPath); err != nil {
		t.Fatalf("MergeSameHeader failed: %v", err)
	}

	// verify output
	of, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer of.Close()

	rows, err := of.GetRows(sheet)
	if err != nil {
		t.Fatalf("failed to read output rows: %v", err)
	}

	// expect header + main row + input row = 3 rows
	if len(rows) != 3 {
		t.Fatalf("unexpected row count: %d", len(rows))
	}

	head := rows[0]
	if len(head) < 3 || head[0] != "ID" || head[1] != "Name" || head[2] != "Age" {
		t.Fatalf("header mismatch: %v", head)
	}

	// input aligned row should be at rows[2]
	inp := rows[2]
	get := func(r []string, i int) string {
		if i < len(r) {
			return r[i]
		}
		return ""
	}
	if get(inp, 0) != "2" || get(inp, 1) != "Bob" || get(inp, 2) != "" {
		t.Fatalf("input row not aligned as expected: %v", inp)
	}
}
