# Go Excel Tool Build Scripts

.PHONY: all clean build test install-deps

# 默认目标
all: build

# 安装依赖
install-deps:
	go mod tidy
	go get fyne.io/fyne/v2/cmd/fyne@latest

# 构建本地版本
build:
	go build -o bin/exceltool cmd/exceltool/main.go

# 构建Windows版本
build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/exceltool-windows-amd64.exe cmd/exceltool/main.go
	GOOS=windows GOARCH=arm64 go build -o bin/exceltool-windows-arm64.exe cmd/exceltool/main.go

# 构建银河麒麟ARM64版本
build-kylin:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build -o bin/exceltool-kylin-arm64 cmd/exceltool/main.go

# 使用fyne-cross构建跨平台版本
build-cross:
	fyne-cross windows -arch=amd64,arm64
	fyne-cross linux -arch=arm64 -release

# 运行测试
test:
	go test -v ./...

# 清理构建文件
clean:
	rm -rf bin/
	rm -rf fyne-cross/

# 安装到系统
install: build
	cp bin/exceltool /usr/local/bin/

# 开发模式运行
run:
	go run cmd/exceltool/main.go

# 打包发布
package: build-cross
	mkdir -p release
	cp fyne-cross/*/exceltool* release/
	cp README.md release/
	cp LICENSE release/

# 帮助信息
help:
	@echo "可用的make命令:"
	@echo "  all          - 构建本地版本 (默认)"
	@echo "  build        - 构建本地可执行文件"
	@echo "  build-windows - 构建Windows版本"
	@echo "  build-kylin  - 构建银河麒麟ARM64版本"
	@echo "  build-cross  - 使用fyne-cross构建跨平台版本"
	@echo "  test         - 运行测试"
	@echo "  clean        - 清理构建文件"
	@echo "  install      - 安装到系统"
	@echo "  run          - 开发模式运行"
	@echo "  package      - 打包发布版本"
	@echo "  install-deps - 安装依赖"
	@echo "  help         - 显示此帮助信息"