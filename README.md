# ExcelTool - 跨平台Excel处理工具

这是一个使用Go语言开发的现代化Excel处理工具，采用Web技术栈实现跨平台支持，完美适配Windows和银河麒麟ARM64架构。

## 技术栈变更说明

**从桌面GUI转向Web技术栈**

考虑到跨平台兼容性和用户体验，我们从传统的桌面GUI框架(Fyne)转向了现代化的Web技术栈：

### 新技术架构
- **后端**: Go语言 + net/http原生Web服务器
- **Excel处理**: [excelize](https://github.com/qiniu/excelize) 库 (功能强大，性能优异)
- **前端界面**: HTML5 + CSS3 + JavaScript
- **UI设计**: 响应式深色主题，与系统配色一致
- **部署方式**: 单一可执行文件，无需额外依赖

### 优势对比
| 特性 | 原桌面GUI | 现Web技术栈 |
|------|-----------|-------------|
| 跨平台兼容性 | ⚠️ 有限 | ✅ 优秀 |
| 系统集成 | ⚠️ 复杂 | ✅ 简单 |
| 用户体验 | ⚠️ 平台差异大 | ✅ 一致体验 |
| 部署复杂度 | ⚠️ 高 | ✅ 极简 |
| 性能表现 | ⚠️ 一般 | ✅ 优异 |

## 核心功能

### 📊 1. 删除行列
- 删除工作表的前N行/后N行
- 删除指定行号范围（支持1,3-5格式）
- 删除前N列/指定列
- 支持批量处理多个文件
- 灵活的输出选项：覆盖原文件或另存为新文件

### 🔗 2. 同构文件合并
- 智能合并多个结构相同的Excel文件
- 自动表头识别与处理
- 批量文件导入支持
- 灵活的输出配置

### 🔗 3. 连接合并
- **强大的关系运算支持**：
  - INNER JOIN (内连接)
  - LEFT JOIN (左连接)
  - RIGHT JOIN (右连接)
  - **NOT IN (A-B)** (差集运算) ⭐
- 多规则连接配置
- 跨工作表连接支持
- 实时结果预览

## 快速开始

### 1. 环境准备
确保已安装Go 1.16+

### 2. 安装依赖
```bash
go mod tidy
```

### 3. 启动应用
```batch
# 推荐方式：使用启动脚本
launch_final.bat

# 或手动编译运行
go build -o exceltool.exe main.go
exceltool.exe
```

### 4. 访问界面
浏览器自动打开 `http://localhost:8080`，即可开始使用所有功能。

## 跨平台编译
```bash
# Windows 64位
GOOS=windows GOARCH=amd64 go build -o exceltool.exe main.go

# 银河麒麟 ARM64
GOOS=linux GOARCH=arm64 go build -o exceltool main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o exceltool main.go
```

## 使用指南

### 🎯 操作流程
1. **启动应用**：双击 `launch_final.bat` 或运行编译后的可执行文件
2. **选择功能**：在左侧菜单选择所需功能模块
3. **配置参数**：根据具体需求设置处理选项
4. **执行操作**：点击相应按钮开始处理
5. **查看结果**：实时日志显示处理进度和结果

### 💡 使用技巧
- **文件管理**：支持拖拽上传和批量文件选择
- **参数保存**：常用配置可保存为模板
- **错误处理**：详细的错误信息帮助快速定位问题
- **性能优化**：大文件处理时建议分批操作

## 开发文档

### 📁 项目架构
```
go-exceltool/
├── main.go              # Web服务器主入口
├── core_processor.go    # Excel处理核心引擎
├── web/
│   ├── style.css        # 深色主题样式
│   └── script.js        # 前端交互逻辑
├── test_files/
│   ├── employee.xlsx    # 员工测试数据
│   ├── resigned.xlsx    # 离职员工数据
│   └── department.xlsx  # 部门信息数据
├── launch_scripts/
│   ├── launch_final.bat # 主启动脚本
│   └── launch_v3.bat    # 备用启动脚本
├── utils/
│   ├── create_test_files.go  # 测试文件生成器
│   └── verify_results.go     # 结果验证工具
└── go.mod              # Go模块配置
```

### 🔧 核心组件说明

**Excel处理器** (`core_processor.go`)
- `ExcelProcessor`: 主处理引擎
- `DeleteConfig`: 删除操作配置
- `JoinRule`: 连接规则定义
- 支持完整的CRUD操作

**Web界面** (`web/`)
- 响应式深色主题设计
- 左侧导航菜单
- 三个核心功能面板
- 实时日志显示

**API接口**
- `/upload`: 文件上传
- `/process/delete`: 删除行列处理
- `/process/merge`: 文件合并
- `/process/join`: 连接合并
- `/download`: 结果下载

### 🧪 测试验证

项目包含完整的测试套件：

```bash
# 核心功能测试
go run core_processor.go

# 结果验证
go run verify_results.go

# 测试文件生成
go run create_test_files.go
```

**测试覆盖**：
- ✅ A NOT IN B 功能验证
- ✅ 删除行列功能测试
- ✅ 文件合并功能验证
- ✅ 跨平台兼容性测试

## 部署说明

### 📦 生产环境部署

**单一可执行文件部署**：
```bash
# 编译优化版本
go build -ldflags="-s -w" -o exceltool main.go
```

**容器化部署**（可选）：
```dockerfile
FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o exceltool main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/exceltool .
EXPOSE 8080
CMD ["./exceltool"]
```

## 重要说明

### ⚠️ 使用注意事项
1. **端口占用**：程序默认使用8080端口，如被占用请修改源码中的端口号
2. **内存管理**：处理大型Excel文件时建议分批操作，避免内存溢出
3. **数据安全**：重要文件操作前请务必备份原始数据
4. **文件格式**：目前支持.xlsx格式，暂不支持.xls格式
5. **浏览器兼容**：推荐使用现代浏览器（Chrome/Firefox/Edge）

### 🔒 安全建议
- 不要在公网环境中暴露此服务
- 处理敏感数据时建议在本地网络环境中使用
- 定期清理临时文件和处理日志

## 版本历史

### v3.0.0 (当前版本)
- ✅ 技术栈升级：从桌面GUI转向Web技术栈
- ✅ 深色主题界面，更好的视觉体验
- ✅ 完整的A NOT IN B功能实现
- ✅ 跨平台支持优化
- ✅ 单一可执行文件部署

### v2.0.0
- Fyne GUI框架实现
- 基础Excel处理功能

### v1.0.0
- Python PySide6版本
- 原始功能实现

## 许可证

MIT License

## 技术支持

如有问题或建议，请提交Issue或联系开发团队。