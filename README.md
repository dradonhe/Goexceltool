# ExcelTool

跨平台 Excel 批处理工具：删除行列、同构合并、连接合并（INNER / LEFT / RIGHT / NOT IN）。

- **后端**：Go 1.x + `net/http` 标准库 + [excelize](https://github.com/xuri/excelize)
- **前端**：HTML + CSS + 原生 JS，通过 `go:embed` 打进二进制
- **零外部依赖**：不依赖 webview / WebKit / GTK / glibc，`CGO_ENABLED=0` 编译后是**完全静态**的单文件
- **零 cgo**：在 Windows 上直接交叉编译出麒麟 / 统信等国产 Linux 架构的产物，不需要 WSL、Docker、zig、交叉编译器

> 运行环境要求：目标 Linux 内核 ≥ 3.2（麒麟 V10 是 4.19，UOS 20 是 4.19/5.10），以及一个桌面浏览器（麒麟/UOS 桌面自带 Firefox）。

---

## 一、功能

### 1. 删除行列
- 删除前 N 行 / 后 N 行 / 指定行（`1,3,5-10`，支持全角逗号、顿号、反向区间 `5-3`）
- 删除前 N 列 / 后 N 列 / 指定列（`A,C,E-G`，也接受数字 `1,3-5`）
- 批量处理多个文件
- 输出模式：另存为新文件（默认加后缀）/ 覆盖同名文件
- **保真**：直接在工作簿副本上调用 `RemoveRow`/`RemoveCol`，样式、公式、列宽、图片、图表、数据验证、条件格式、其它工作表全部保留；合并单元格、超链接的引用由 excelize 自动调整

### 2. 同构合并
- 以**主文件表头**为准，其它文件按**表头名**对齐，缺少的列留空
- 输入流式读取 + 输出 `StreamWriter` 流式写入，内存占用与总行数基本无关
- 表头为空/重复会自动跳过并给出警告；缺少目标工作表的文件会被跳过并报告

### 3. 连接合并
| 类型 | 语义 |
|---|---|
| `INNER JOIN` | 只保留两边键都匹配上的行 |
| `LEFT JOIN` | 保留左表全部行，右表无匹配时留空 |
| `RIGHT JOIN` | 保留右表全部行，左表无匹配时留空 |
| `NOT IN (A-B)` | 差集：左表键不在右表中的行（只输出左表列） |

- 支持**多规则链式连接**：第 1 条规则作用于基表，之后每条规则作用于上一步的结果
- 连接键按列名匹配（先精确、再忽略大小写、最后支持数字列号），值做 `TrimSpace` 归一化
- 右列重名自动加后缀并给出警告
- 输出字段可留空（全部列），也可写 `table1.sheet1.*`、`table2.列名`、`列名`

---

## 二、构建

### Windows（推荐，一键出全部架构）

```powershell
# 需要 Go 1.23+ 在 PATH 中
.\scripts\build.ps1 -Version 3.1.0
```

脚本会：
1. 强制 `CGO_ENABLED=0`
2. 扫描依赖里是否有任何带 C 代码的包，有就**直接失败**
3. 依次构建 linux/amd64、linux/arm64、linux/loong64、linux/mips64le、windows/amd64、windows/arm64
4. 用 `scripts/check-elf.ps1` 逐个校验 ELF，**不是完全静态就失败**
5. 生成 `bin/SHA256SUMS.txt`

### Linux / macOS / WSL

```bash
make linux VERSION=3.1.0     # 国产 Linux 四个架构
make windows                 # Windows
make check                   # file + ldd 校验静态性
make test                    # go test ./...
```

### 手工构建

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -tags "netgo osusergo" \
  -ldflags "-s -w -X main.version=3.1.0" \
  -o bin/exceltool-linux-arm64 ./cmd/exceltool
```

> 不要加 `-buildmode=pie`：会给产物加回动态加载器依赖。

### 在目标机验证

```bash
chmod +x exceltool-linux-arm64
ldd ./exceltool-linux-arm64          # 期望：not a dynamic executable
./exceltool-linux-arm64 --no-browser  # 控制台会打印访问地址
```

---

## 三、运行

```bash
./exceltool                 # 自动挑空闲端口并打开浏览器
./exceltool -no-browser     # 不自动开浏览器（服务器/无桌面环境）
./exceltool -port 18080     # 指定端口
./exceltool -output /data/输出   # 指定默认输出目录
./exceltool -version
```

服务只监听 `127.0.0.1`，不会暴露到局域网。

---

## 四、输出路径规则（重要）

浏览器出于安全限制**不会**把文件的绝对路径告诉后端，所以「输出目录」由用户填写，规则如下：

| 你填的内容 | 结果 |
|---|---|
| 留空 | 可执行文件旁的 `output/` 目录 |
| `/data/out` | `/data/out/<原文件名>.xlsx`（不存在会自动创建） |
| `/data/out/` | 同上（结尾斜杠无所谓） |
| `/data/out/结果.xlsx` | 单文件时精确输出到这个文件 |
| `" /data/out "` | 自动去掉首尾空白和引号 |
| 目录里含空格/中文 | 原样保留，不会被改写 |
| 批量处理时填了 `.xlsx` | 报错：批量模式必须给目录 |

实现要点（`internal/app/paths.go`）：

- 目录与文件名一律用 `filepath.Join` 拼接，绝不字符串相加，因此不会出现 `/file abc.xlsx` 这种把目录和文件名用空格连起来的历史问题
- 上传文件名先做 `SanitizeFileName`：同时按 `/` 和 `\` 切分取最后一段，过滤控制字符和 Windows 非法字符，修正 `CON/PRN/...` 保留名
- 写盘用「临时文件 + 重命名」，覆盖同名文件时即使中途失败也不会留下半个文件
- 最终路径统一转成绝对路径并回显在界面上

---

## 五、HTTP 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/config` | 默认输出目录、版本、平台 |
| POST | `/api/sheet-info` | 读取工作簿表名 / 行列数 / 表头 |
| POST | `/api/delete-rows` | 删除行列（multipart） |
| POST | `/api/merge` | 同构合并（multipart） |
| POST | `/api/join` | 连接合并（multipart + JSON `spec`） |
| GET | `/api/download?path=` | 下载结果（仅限白名单目录） |
| POST | `/api/reveal?path=` | 在文件管理器中打开（仅本机页面） |
| POST | `/api/exit` | 退出程序（仅本机页面） |

安全措施：
- 只监听回环地址
- `/api/exit`、`/api/reveal` 校验 `Origin`/`Referer`，第三方网页无法驱动
- 上传限制 1 GiB、单次最多 200 个文件
- 下载只允许本次任务写出的目录和默认输出目录，`..` 越权会被拒绝

---

## 六、目录结构

```
cmd/exceltool/main.go      # 入口：参数、端口、打开浏览器
internal/app/paths.go      # 输出路径解析（本项目的核心修复点）
internal/app/server.go     # 路由与处理器
pkg/excel/delete.go        # 删除行列（copy-then-mutate）
pkg/excel/merge.go         # 同构合并（流式）
pkg/excel/join.go          # 四类连接 + 链式规则
pkg/excel/range.go         # 行列范围解析
web/                       # index.html / script.js / style.css（go:embed）
scripts/build.ps1          # 交叉编译 + 静态性校验
scripts/check-elf.ps1      # ELF 校验器
test/integration_test.go   # 端到端 HTTP 测试
docs/                      # 技术选型与静态编译验证报告
```

---

## 七、限制与说明

1. **只支持 `.xlsx` / `.xlsm`**。`.xls`（97-2003 二进制格式）excelize 不支持；如需支持，必须外挂转换器（会破坏「零依赖」）。
2. **excelize 会把整个工作簿读进内存**，超大文件（>200 MB）请先在 Excel 里拆分。
3. 合并 / 连接的输出是**新工作簿**，单元格按文本写入，源文件里的样式不会带过来（删除行列走的是副本，样式保留）。
4. 界面依赖浏览器。麒麟/UOS 桌面自带 Firefox；若在纯命令行服务器上运行，用 `-no-browser` 后从别的机器访问会失败（服务绑定回环地址），这是有意的安全设计。

---

## 八、测试

```bash
go test ./...
```

覆盖：
- 范围解析（全角符号、反向区间、越界）
- 删除行列后端到端，以及**样式/公式/合并单元格/列宽是否保留**
- 合并的表头对齐与缺列处理
- 四种连接类型、链式连接、输出字段筛选
- 输出路径：带空格、中文、结尾斜杠、精确文件路径、批量模式拒绝文件路径、越权下载、外部 Origin 调用退出

## 许可证

MIT
