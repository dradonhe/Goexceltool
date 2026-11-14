# ExcelTool 技术选型评估：继续 Go 还是改 Rust

> 评估对象：`https://github.com/dradonhe/Goexceltool`（2026-02-26 提交 `bf0e286`）
> 核心约束：功能不减少、Windows 上交叉编译出 Linux（含国产 Linux）、产物零系统/语言依赖
> 结论：**继续用 Go，不要迁移 Rust。** 真正要解决的不是语言，而是「仓库残缺 + 架构里残留的 CGO 依赖」。

---

## 一、先说结论（可执行版）

1. **语言不动**。Go + `excelize`（纯 Go）+ `net/http` 已经完全满足「Windows 交叉编译出国产 Linux 静态单文件」这一硬指标，且是当前生态里成本最低的路径。
2. **必须去掉 Fyne**。`go.sum` 里 `fyne.io/fyne/v2`、`go-gl/gl`、`go-gl/glfw` 是唯一会把 `CGO_ENABLED` 逼成 1 的东西，也是 `Makefile` 里出现 `aarch64-linux-gnu-gcc` 和 `fyne-cross` 的原因。Web UI 架构已经绕开了它，只是依赖没收干净。
3. **仓库当前编译不过**，缺 `go.mod`、缺后端、缺 `index.html`；「连接合并（INNER/LEFT/RIGHT/NOT IN）」在源码里根本不存在。这些跟选 Go 还是 Rust 无关，换语言一样要做。
4. **Rust 方案会在这台机器上直接卡住**：已装 `rustc 1.93.0`，但只有 `x86_64-pc-windows-msvc` 一个 target，没有 `zig`、没有 Linux 版 WSL 发行版、没有 Docker。要产出 Linux 产物得先补交叉链接器和 sysroot/zig 工具链；而 Go 只需要装一个 Go。

---

## 二、仓库现状体检（有据可查）

### 2.1 致命项：构建链断了

| 问题 | 证据 |
|---|---|
| 没有 `go.mod`，只有 `go.sum` | 仓库根目录只有 `go.sum`（66 KB，来自另一个模块图）；`go build` 第一步就失败 |
| `main.go` 是可丢弃的脚本，不是程序入口 | `main.go:17` 硬编码 `e:\Texcel\excelfiles\核定待办0101-0131.xlsx`，跑完打印统计就结束 |
| 后端 HTTP 服务完全缺失 | 全部 1295 行 Go 代码里 **0 处** `http.` 引用 |
| 前端调的接口没有任何实现 | `web/script.js:300` `/api/delete-rows`、`:422` `/api/sheet-info`、`:462` `/api/exit` |
| 而已经发出去的 exe 用的是另一套接口 | `exceltool.exe` 里只有 `/api/upload`、`/api/process`、`/api/download`、`/api/exit` —— **前后端契约对不上，是两个版本的碎片混在一个仓库** |
| 缺少 `index.html` | `web/` 下只有 `style.css`、`script.js` |
| 合并/连接在前端是假的 | `script.js:328-340` `executeMerge` 是 `setTimeout(…3000)`；`:342-360` `executeJoin` 同理，只弹个「完成」 |
| `Makefile` 指向不存在的文件 | `Makefile:15` `cmd/exceltool/main.go`、`:11` `fyne` CLI、`:24` `CC=aarch64-linux-gnu-gcc`、`:27` `fyne-cross` |
| 测试跑不起来 | `tests/core`、`tests/excel`、`tests/utils` 没有 `.go` 后缀，go 工具链直接忽略；`test/ui_integration_test.go:12` 导入 `github.com/user/go-exceltool/...`，该模块不存在 |
| 二进制入库 | `exceltool.exe`、`exceltool_fixed.exe` 各 8.4 MB 进了 Git |
| 同一函数两份实现 | `core_merge.go:15` 与 `pkg/excel/handler.go:366` 的 `MergeSameHeader` 逐行重复（仅注释不同） |

### 2.2 「单文件零依赖」其实已经名不副实

- `exceltool.exe`（Go 1.25.6 构建）里能搜到内嵌的 `index.html`（含 `delete-first-rows`、`delete-sheet-select` 等 id），但**搜不到 `updateStatusBar`、`addLog`、`fileLists`、`webkitdirectory`** —— 说明 `script.js` / `style.css` 没有 `go:embed`，是运行时从磁盘 `/static/` 读的。
- 也就是说：把 exe 单独拷到麒麟机器上，页面能开，但按钮全是死的（JS 404）。README 第 16 行「单一可执行文件，无需额外依赖」与事实不符。
- 另外内嵌 HTML 是 **GBK 字节**却声明 `<meta charset="UTF-8">`，中文会乱码（我在二进制里按 GBK 解码才读出正常中文）。这是发出去的那个版本的实际缺陷。

### 2.3 Excel 处理逻辑的保真度问题（换语言前必须先定基线）

`pkg/excel/ops.go:12`、`MergeSameHeader`、`main.go:43` 的做法都是：
`GetRows()` 读成 `[][]string` → 在内存里删行列/对齐 → `NewFile()` 重建 → `SetCellValue()` 逐格写。

后果（对真实业务表格来说是硬伤）：

- **丢格式**：字体、边框、底色、列宽行高、条件格式、数据验证、冻结窗格、打印区域全没了。
- **丢公式**：公式被 `GetRows` 取成结果值，写回去是死值。
- **日期变字符串**：`GetRows` 返回显示值，写回后 `2024-01-01` 变成文本，后续计算/筛选失效。
- **合并单元格不跟着挪**：删行后合并区域仍指向旧坐标，表格看起来错乱。
- **多 Sheet 丢失**：`ops.go:58-60` 只 `NewSheet(sheetName)` 一个表，源文件其它 Sheet、图表、图片、透视表全部消失。
- `handler.go:200-230` 的 `shiftRowsUp` 兜底路径更糟：按字符串搬行，`len(rows[0])` 在空表上还会 panic。

正确做法（Go 里一行不用多写复杂逻辑）：**把源文件直接拷贝到输出路径，然后对副本调用 `RemoveRow` / `RemoveCol`**。excelize 会保持样式、公式、列宽、图片，只需额外重映射合并单元格区域。这比现在的「读-重建-写」又快又保真。

### 2.4 其它已定位的缺陷

| 位置 | 问题 |
|---|---|
| `handler.go:114` vs `:133` | `DeleteRows` 收 1-based，`DeleteColumns` 收 0-based（内部 `colIndex+1`），同一个对象两套坐标，调用方必错 |
| `handler.go:180-188` | `CopySheet` 把 `GetSheetIndex` 返回 0 当作失败（第一个 Sheet 的合法索引就是 0） |
| `handler.go:124-126` | `RemoveRow` 失败就静默走兜底，用户永远不知道发生了降级 |
| `handler.go:216` | 写入时 `CoordinatesToCellName(..., i)` 与 `rows[nextRow-1]` 混用，逻辑靠巧合正确 |
| `web/script.js:84-103` | 「选择输出文件夹」用 `webkitdirectory` 只能拿到相对路径，拿不到绝对路径，后端没法写 |
| `main.go:17`、`test_harness.go:20`、`test/ui_integration_test.go:48` | 硬编码 `E:\Texcel\excelfiles`，换机器必挂 |

---

## 三、需求 → 方案 逐条对照

| 你的要求 | Go（现状 + 清理后） | Rust（需重写） |
|---|---|---|
| Windows 上编 Linux | ✅ `GOOS=linux GOARCH=amd64 go build`，**不需要链接器、不需要 sysroot、不需要 WSL/Docker** | ⚠️ 需要 Linux 目标链接器：`zig` + `cargo-zigbuild`，或 `cross`+Docker，或 WSL；本机三者都没有 |
| 产物零系统依赖 | ✅ `CGO_ENABLED=0` 即静态 ELF，不链 glibc，`ldd` 显示 `not a dynamic executable` | ✅ 可以做到（`*-unknown-linux-musl` + `crt-static`），但前提是上面的交叉工具链先搭好 |
| 国产 Linux x86_64（麒麟 V10 / 统信 UOS） | ✅ 一个命令 | ✅ 一个 target（需链接器） |
| 国产 Linux arm64（飞腾 / 鲲鹏） | ✅ | ✅（需链接器） |
| 国产 Linux loong64（龙芯 3A5000+） | ✅ `GOARCH=loong64`，Go 1.19 起一等公民，纯 Go 直接出包 | ❌ `loongarch64-unknown-linux-gnu` 是 Tier 3：无预编译 std，要 nightly + `-Zbuild-std`，维护成本极高 |
| 国产 Linux mips64le（龙芯 3A3000/4000 存量机） | ✅ `GOARCH=mips64le` | ❌ 不支持 |
| Excel 读写保真（样式/公式/合并） | ✅ `excelize` 是 Go 事实标准（6 年+、issue 处理充分），支持流式读写、样式、图片、图表、条件格式 | ⚠️ 要拼 2-3 个 crate：`calamine`（只读）、`rust_xlsxwriter`（只写）、`umya-spreadsheet`（读写但覆盖不全）；**输出语义与 excelize 不同，等于全部产物重新验收** |
| 前端零改动 | ✅ 本来就不用改 | ✅ 本来就不用改 |
| 团队/机器现状 | ⚠️ 本机没装 Go（装一个即可） | 本机有 rustc 但只能编 Windows-MSVC |

**一句话**：硬指标上 Go 不只是「够用」，而是「开箱即用」；Rust 是「理论上可以，工程上要先搭一套交叉工具链，且国产长尾架构基本劝退」。

---

## 四、为什么明确不建议改 Rust

按权重排序：

1. **交叉编译是这次的第一需求，而这恰好是 Go 的最强项、Rust 的最弱项。** Go 的交叉编译是「改两个环境变量」；Rust 的交叉编译是「装交叉链接器 + 准备 sysroot + 处理 libc 版本」。你要的正是「在 Windows 上一键出国产 Linux 包」，Go 原生就是这个体验。用 Rust 等于为了换语言，先给自己加一道最容易出事的工序。
2. **信创的长尾架构，Rust 生态覆盖不住。** 龙芯 loong64、老龙芯 mips64le 在政企/事业单位存量不小。Go 全部一等公民、纯 Go 出静态包；Rust 在 Tier 3 上要 nightly 构建 std，等于把「能不能交付」押在 nightly 不回归上。
3. **Excel 是最不值得重写的那部分。** 业务价值在「删行列 / 同构合并 / 四类连接」这些规则上，代码量不到 600 行，任何语言都能写；真正难的是 XLSX 的 ZIP+XML+样式+公式+合并单元格细节。excelize 已经把这些坑踩完了，Rust 侧没有等价物。重写 Excel 层 = 用你的真实业务表格重新做一遍验收，风险远大于收益。
4. **现有资产是 Go，而且线上跑过。** 仓库里 1295 行 Go、能跑通的 `exceltool.exe`（Go 1.25.6）都在。Rust 方案是 100% 从零，包括已经踩过的 XLSX 兼容性坑要再踩一遍。
5. **Rust 的优势在这个项目上用不上。** 峰值内存由文件大小决定（都是 O(数据量)）、CPU 不是瓶颈（瓶颈是磁盘和 Excel 解析）、单文件体积 Go 约 10-12 MB 对桌面工具完全可接受。省下来的 5 MB 换不来 3 周工期和一套交叉工具链。

**什么情况下才该选 Rust**（诚实边界）：
- 交付物对体积/内存有硬指标（比如 <5 MB、嵌入式设备）；
- 需要极致解析吞吐（百万行级实时）；
- 团队只有 Rust 能力，没有 Go；
- 未来要做原生 GUI 或 WASM 端并复用同一套核心。
以上都不成立。

---

## 五、Go 路线落地计划

### P0 — 先让它能编译（0.5 天）

```
excel-tool/
├── go.mod                     # module exceltool; go 1.23
├── cmd/exceltool/main.go      # 真正的入口：flag(port/open) → server
├── internal/
│   ├── excel/                 # handler.go + ops.go（去掉重复的 MergeSameHeader）
│   ├── join/                  # INNER / LEFT / RIGHT / NOT IN
│   └── server/                # 路由、上传、下载、退出
├── web/                       # index.html + style.css + script.js
│       └── embed.go           # //go:embed → 单文件
└── .gitignore                 # *.exe / bin/ / result*.xlsx
```

- 新建 `go.mod`，`go mod tidy` 重建 `go.sum`（旧的 66 KB 是别的模块图留下的，含 Fyne，直接丢）。
- **依赖只留 `github.com/xuri/excelize/v2`**，确保 `go.sum` 里不再出现 `fyne.io`、`go-gl/*`。
- `Makefile` 重写为 `GOOS/GOARCH + CGO_ENABLED=0` 矩阵，删掉 `fyne-cross` 与 `aarch64-linux-gnu-gcc`。
- **零 CGO 闸门**：CI/本地构建强制 `CGO_ENABLED=0`，一旦有人引入需要 C 的库，构建立刻失败，而不是等打包到麒麟上才发现。

### P1 — 功能补齐（2-3 天）

- 后端四个接口：`/api/upload`（落临时目录，大小上限）、`/api/process`（按 panel 分发）、`/api/download`（只允许下载本次产出、文件名白名单）、`/api/exit`（仅回环）。
- 连接合并四种语义，按 `tests/core` 里的 `JoinRule` 字段（`LeftTable/LeftSheet/LeftColumn/JoinType/RightTable/...`）落成真实实现：
  - `INNER`、`LEFT`、`RIGHT`（缺失侧补空）、`NOT IN`（左表键不在右表键集合，右表列不出现在结果）。
  - 键比较要做归一化：`TrimSpace`、数字/文本区分、大小写策略可配置（否则 `"001"` 与 `1`、全角空格会坑人）。
- `index.html` 直接从 `exceltool.exe` 里回收（内嵌 13,233 字节，GBK 需转 UTF-8），比照着重画便宜得多。
- 前后端契约统一到一套（现在 `script.js` 打的是 `/api/delete-rows`，exe 是 `/api/process`，二选一，写进 README）。

### P2 — 保真度与正确性（1-2 天，性价比最高）

- 删行列改成 **copy-then-mutate**：`os.Copy` 源文件 → `excelize.OpenFile` → `RemoveRow` / `RemoveCol` → 重映射合并单元格 → `Save`。样式、公式、列宽、图片一次全保住。
- 统一坐标约定：对外一律 1-based，内部转换只在一处做。
- 多 Sheet：默认处理用户选中的 Sheet，其它 Sheet 原样保留（现在是全丢）。
- 大文件：`Rows()` 流式读 + `StreamWriter` 流式写，避免 100 MB 文件把内存打满。
- 空表/越界/非法范围串（`1,3-5`、`A,C,E-G`）全部走参数校验，不 panic。

### P3 — 交付工程化（1 天）

- GitHub Actions 矩阵，一次出齐：`linux/amd64`、`linux/arm64`、`linux/loong64`、`linux/mips64le`、`windows/amd64`、`windows/arm64`，附 SHA256。
- `go mod vendor` 或预热 `GOMODCACHE`，保证内网/离线环境能构建（信创交付常态）。
- 构建参数：`-trimpath -ldflags "-s -w -X main.version=... -X main.commit=..."`，`GOFLAGS=-buildvcs=false`。
- `import _ "time/tzdata"`：否则静态包在没有 `/usr/share/zoneinfo` 的最小化系统上 `LoadLocation` 会失败（国产化精简镜像常见）。
- 可复现构建验证脚本：`file` 看 `statically linked`，`ldd` 看 `not a dynamic executable`。

### 构建矩阵（P0 完成后即可用）

```powershell
# 一次出全部国产 Linux 包，无需任何交叉链接器
$targets = @(
  @{os='linux';   arch='amd64'},    # 麒麟 V10 / UOS x86_64
  @{os='linux';   arch='arm64'},    # 飞腾 / 鲲鹏
  @{os='linux';   arch='loong64'},  # 龙芯 3A5000+
  @{os='linux';   arch='mips64le'}, # 老龙芯 3A3000/4000
  @{os='windows'; arch='amd64'}
)
foreach ($t in $targets) {
  $env:CGO_ENABLED='0'; $env:GOOS=$t.os; $env:GOARCH=$t.arch
  $ext = if ($t.os -eq 'windows') { '.exe' } else { '' }
  go build -trimpath -buildvcs=false -ldflags "-s -w" `
    -o "bin/exceltool-$($t.os)-$($t.arch)$ext" ./cmd/exceltool
}
```

Rust 等价物（对照即可看出差距）：先 `rustup target add aarch64-unknown-linux-musl`，再装 `zig`/`cargo-zigbuild` 配链接器，loong64 还得 `+nightly -Zbuild-std`。

---

## 六、成本对照

| | 继续 Go（清理 + 补全） | 改 Rust（重写） |
|---|---|---|
| 语言/框架重写 | 0 | 全部（HTTP + Excel + 连接引擎） |
| Excel 能力对齐 | 直接复用 excelize | 拼 3 个 crate + 语义差异适配 |
| 交叉工具链 | 装 Go，完成 | zig/cross/WSL + musl target（loong64 还需 nightly） |
| 真实表格回归验收 | 只验改动点 | 全量重验（输出可能不同） |
| 估算 | **4-6 人日** | **25-35 人日**，且多一个长期维护项 |
| 交付风险 | 低（路径已验证） | 中高（国产架构 + Excel 保真双重不确定） |

---

## 七、还没解决的风险（与语言无关，需产品决策）

1. **`.xls` 不支持**：README 第 198 行已声明。excelize 只吃 xlsx；Rust 也一样。若业务确有 `.xls`，要么换 `.xlsx`，要么引入转换器（LibreOffice 无头模式）—— 但这直接违背「零依赖」，必须拍板。
2. **浏览器是事实上的运行依赖**：Web UI 方案决定了必须有浏览器。国产化环境常见 Firefox ESR / 奇安信 / 360，`webkitdirectory`、`fetch`、`FormData` 都支持，但 `window.close()`（`script.js:468`）大概率被拦，退出流程要改成「提示已退出，可手动关页」。
3. **8080 端口**（README 195 行）在客户机上经常被占。改成「从 8080 起找空闲端口 + 控制台打印 URL + `--port` 覆盖」。
4. **安全面**：服务必须只绑 `127.0.0.1`；`/api/exit` 要么删掉，要么校验 `Origin`，否则用户随便打开一个网页就能让你的进程退出；上传要限大小、产出要限路径（防目录穿越）。
5. **excelize 并发**：同一个 `*excelize.File` 不能并发写。多文件批量处理时要么串行，要么每文件一个实例 + 限流。
6. **合并单元格重映射**：`RemoveRow` 不会自动搬合并区域，必须自己按删除区间重算，这是 copy-then-mutate 方案里唯一需要小心的地方。

---

## 八、如果只做三件事

1. 建 `go.mod`、删 Fyne、`Makefile` 改成 `CGO_ENABLED=0` 矩阵 —— 当天就能在 Windows 上产出麒麟可跑的静态单文件。
2. `index.html` 从 `exceltool.exe` 回收，`web/` 用 `go:embed` 打进二进制 —— 兑现「单文件」承诺，顺手修掉 GBK 乱码。
3. 删行列改成 copy-then-mutate —— 用户最直观的「格式没丢」体验，改动最小、收益最大。

**不要为了「跨平台」换语言。Go 已经把这件事做完了；这个仓库缺的是把 Fyne 的尾巴切干净、把丢掉的 600 行补回来。**
