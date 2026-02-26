package main

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func main() {
	// 读取生成的在职员工文件
	f, err := excelize.OpenFile("active_employees.xlsx")
	if err != nil {
		fmt.Printf("无法打开文件: %v\n", err)
		return
	}

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		fmt.Printf("无法读取数据: %v\n", err)
		return
	}

	fmt.Println("=== 在职员工列表 (A NOT IN B 结果) ===")
	fmt.Printf("总行数: %d (包含表头)\n\n", len(rows))

	// 显示表头
	if len(rows) > 0 {
		fmt.Println("表头:", rows[0])
		fmt.Println(strings.Repeat("-", 80))
	}

	// 显示数据行
	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}
		fmt.Printf("第%d行: ", i)
		for _, cell := range row {
			fmt.Printf("%s\t", cell)
		}
		fmt.Println()
	}

	// 验证结果正确性
	fmt.Println("\n=== 结果验证 ===")
	fmt.Println("原始员工表: 8条记录 (员工ID: 1001-1008)")
	fmt.Println("离职员工表: 3条记录 (员工ID: 1002, 1005, 1008)")
	fmt.Println("预期在职员工: 5条记录 (员工ID: 1001, 1003, 1004, 1006, 1007)")
	fmt.Printf("实际结果: %d条记录\n", len(rows)-1)

	// 检查具体员工ID
	activeIDs := make(map[string]bool)
	for i, row := range rows {
		if i > 0 && len(row) > 0 {
			activeIDs[row[0]] = true
		}
	}

	expectedIDs := []string{"1001", "1003", "1004", "1006", "1007"}
	allCorrect := true

	for _, expectedID := range expectedIDs {
		if !activeIDs[expectedID] {
			fmt.Printf("❌ 缺少预期员工: %s\n", expectedID)
			allCorrect = false
		}
	}

	if allCorrect {
		fmt.Println("✅ A NOT IN B 功能测试通过！结果完全正确")
	} else {
		fmt.Println("❌ 结果有误")
	}
}
