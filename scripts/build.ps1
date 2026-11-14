# 静态交叉编译脚本：在 Windows 上一次产出全部国产 Linux 架构
#
# 设计原则：
#   1. 强制 CGO_ENABLED=0 —— 这是「不依赖 glibc / 不需要交叉链接器」的唯一开关
#   2. 每个 Linux 产物构建后立刻做 ELF 校验，不是静态就直接构建失败
#
# 用法：
#   .\scripts\build.ps1                     # 全部目标
#   .\scripts\build.ps1 -Targets linux-amd64,linux-arm64
#   .\scripts\build.ps1 -Package ./cmd/exceltool
#
param(
    [string]$Package = "./cmd/exceltool",
    [string]$OutDir  = "bin",
    [string]$Version = "dev",
    [string[]]$Targets = @(
        "linux-amd64",     # 麒麟 V10 / 统信 UOS，x86_64
        "linux-arm64",     # 飞腾 / 鲲鹏
        "linux-loong64",   # 龙芯 3A5000 及以上
        "linux-mips64le",  # 存量龙芯 3A3000/4000
        "windows-amd64",
        "windows-arm64"
    )
)

$ErrorActionPreference = "Stop"

# ---------- 0. 前置检查 ----------
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) { throw "找不到 go，请先把 Go 加入 PATH" }

if (-not (Test-Path (Join-Path $PSScriptRoot "..\go.mod"))) {
    throw "仓库根目录缺少 go.mod。先执行：go mod init exceltool && go mod tidy"
}

# 硬闸门：任何情况下都不许开启 cgo
$env:CGO_ENABLED = "0"
$env:GOFLAGS = "-buildvcs=false"
$env:GODEBUG = ""

# 依赖检查：列出所有会引入 C 代码的依赖（正常应为空）
$cgoDeps = & go list -deps -f "{{if .CgoFiles}}{{.ImportPath}}{{end}}" $Package 2>$null | Where-Object { $_ }
if ($cgoDeps) {
    Write-Host "编译失败：以下依赖包含 C 代码，会产生动态链接，必须替换：" -ForegroundColor Red
    $cgoDeps | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    throw "存在 cgo 依赖"
}

New-Item -ItemType Directory -Force $OutDir | Out-Null
$ldflags = "-s -w -X main.version=$Version"
$built = @()

# ---------- 1. 逐个目标构建 ----------
foreach ($t in $Targets) {
    $parts = $t.Split("-")
    $env:GOOS = $parts[0]
    $env:GOARCH = $parts[1]

    $name = "exceltool-$t"
    if ($env:GOOS -eq "windows") { $name += ".exe" }
    $out = Join-Path $OutDir $name

    Write-Host ("==> {0,-14} {1}" -f $t, $out) -ForegroundColor Cyan
    & go build -trimpath -tags "netgo osusergo" `
        -ldflags "$ldflags -extldflags '-static'" `
        -o $out $Package
    if ($LASTEXITCODE -ne 0) { throw "构建失败: $t" }

    $built += $out
}

# ---------- 2. 静态性校验 ----------
$checker = Join-Path $PSScriptRoot "check-elf.ps1"
$failed = @()
foreach ($f in $built) {
    if ($f -match "\.exe$") { continue }
    & $checker -Path (Resolve-Path $f) | Out-Null
    if ($LASTEXITCODE -ne 0) { $failed += $f }
}

if ($failed) {
    Write-Host "`n以下产物不是完全静态，禁止交付：" -ForegroundColor Red
    $failed | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    throw "静态性校验失败"
}

# ---------- 3. 清单与校验和 ----------
$sums = foreach ($f in $built) {
    $h = (Get-FileHash $f -Algorithm SHA256).Hash.ToLower()
    $size = (Get-Item $f).Length
    "{0}  {1}  {2}" -f $h, $size, (Split-Path $f -Leaf)
}
$sums | Set-Content (Join-Path $OutDir "SHA256SUMS.txt") -Encoding utf8

Write-Host "`n全部产物均已通过静态校验：" -ForegroundColor Green
Get-ChildItem $built | ForEach-Object {
    Write-Host ("  {0,-28} {1,10:N0} bytes" -f $_.Name, $_.Length)
}
Write-Host "`n校验和已写入 $OutDir\SHA256SUMS.txt"
Write-Host "提示：用 zip 分发会丢失可执行权限，到 Linux 后需 chmod +x；或用 tar.gz 打包。" -ForegroundColor Yellow
