# ExcelTool 构建脚本
#
# 首选：Windows 上用 PowerShell 脚本（会做静态性校验）
#   .\scripts\build.ps1 -Version 3.1.0
#
# 备用：Linux / macOS / WSL 上用 make
#   make linux VERSION=3.1.0
#
# 所有目标都强制 CGO_ENABLED=0：
#   产物不链接 glibc、不需要动态加载器，Windows 上可直接交叉编译出国产 Linux 包。

BINARY   := exceltool
VERSION  ?= dev
OUT      := bin
PKG      := ./cmd/exceltool
LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  := -trimpath -buildvcs=false -tags "netgo osusergo"

.PHONY: all build linux windows darwin test vet tidy fmt clean help

all: build

## 本机构建
build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY) $(PKG)

## 国产 Linux 全套架构
linux:
	@mkdir -p $(OUT)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64   go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-linux-amd64   $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64   go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-linux-arm64   $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-linux-loong64 $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-linux-mips64le $(PKG)

## Windows
windows:
	@mkdir -p $(OUT)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-windows-amd64.exe $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-windows-arm64.exe $(PKG)

## macOS
darwin:
	@mkdir -p $(OUT)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-darwin-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(OUT)/$(BINARY)-darwin-arm64 $(PKG)

## 校验产物是否为完全静态（Linux 上可直接判定）
check:
	@for f in $(OUT)/$(BINARY)-linux-*; do \
		echo "== $$f"; \
		file "$$f"; \
		ldd "$$f" 2>&1 | head -1; \
	done

test:
	CGO_ENABLED=0 go test ./...

vet:
	CGO_ENABLED=0 go vet ./...

tidy:
	go mod tidy

fmt:
	gofmt -l -w .

clean:
	rm -rf $(OUT) output

help:
	@echo "可用目标："
	@echo "  build    本机构建（CGO_ENABLED=0）"
	@echo "  linux    国产 Linux：amd64 / arm64 / loong64 / mips64le"
	@echo "  windows  Windows：amd64 / arm64"
	@echo "  darwin   macOS：amd64 / arm64"
	@echo "  check    检查 Linux 产物是否完全静态"
	@echo "  test     go test ./..."
	@echo "  vet      go vet ./..."
	@echo "  clean    清理 bin/ 与 output/"
