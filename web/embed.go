// Package web 把前端静态资源编译进二进制。
// 只依赖标准库的 embed，不涉及任何 webview / WebKit / CEF。
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html script.js style.css
var assets embed.FS

// FS 返回前端资源；直接挂在 "/" 上即可。
func FS() fs.FS { return assets }
