package main

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

func main() {
	fmt.Println("=== 创建测试Excel文件 ===")

	// 创建测试文件A (员工表)
	createEmployeeFile()

	// 创建测试文件B (离职员工表)
	createResignedFile()

	// 创建测试文件C (部门表)
	createDepartmentFile()

	fmt.Println("测试文件创建完成！")
	fmt.Println("文件位置:")
	fmt.Println("- employee.xlsx (员工表)")
	fmt.Println("- resigned.xlsx (离职员工表)")
	fmt.Println("- department.xlsx (部门表)")
}

func createEmployeeFile() {
	f := excelize.NewFile()

	// 创建员工数据
	data := [][]interface{}{
		{"员工ID", "姓名", "部门", "薪资", "入职日期"},
		{1001, "张三", "技术部", 8000, "2023-01-15"},
		{1002, "李四", "销售部", 7500, "2023-02-20"},
		{1003, "王五", "技术部", 9000, "2023-03-10"},
		{1004, "赵六", "人事部", 6500, "2023-01-05"},
		{1005, "钱七", "财务部", 7000, "2023-04-12"},
		{1006, "孙八", "技术部", 8500, "2023-02-28"},
		{1007, "周九", "销售部", 7200, "2023-03-22"},
		{1008, "吴十", "人事部", 6800, "2023-05-01"},
	}

	for i, row := range data {
		for j, cell := range row {
			cellName, _ := excelize.CoordinatesToCellName(j+1, i+1)
			f.SetCellValue("Sheet1", cellName, cell)
		}
	}

	f.SaveAs("employee.xlsx")
	fmt.Println("✓ employee.xlsx 创建完成")
}

func createResignedFile() {
	f := excelize.NewFile()

	// 创建离职员工数据
	data := [][]interface{}{
		{"员工ID", "姓名", "离职日期", "离职原因"},
		{1002, "李四", "2023-06-15", "个人发展"},
		{1005, "钱七", "2023-07-01", "家庭原因"},
		{1008, "吴十", "2023-06-30", "健康原因"},
	}

	for i, row := range data {
		for j, cell := range row {
			cellName, _ := excelize.CoordinatesToCellName(j+1, i+1)
			f.SetCellValue("Sheet1", cellName, cell)
		}
	}

	f.SaveAs("resigned.xlsx")
	fmt.Println("✓ resigned.xlsx 创建完成")
}

func createDepartmentFile() {
	f := excelize.NewFile()

	// 创建部门数据
	data := [][]interface{}{
		{"部门代码", "部门名称", "部门经理", "预算"},
		{"TECH", "技术部", "张三", 500000},
		{"SALES", "销售部", "李四", 300000},
		{"HR", "人事部", "王五", 200000},
		{"FIN", "财务部", "赵六", 250000},
	}

	for i, row := range data {
		for j, cell := range row {
			cellName, _ := excelize.CoordinatesToCellName(j+1, i+1)
			f.SetCellValue("Sheet1", cellName, cell)
		}
	}

	f.SaveAs("department.xlsx")
	fmt.Println("✓ department.xlsx 创建完成")
}
