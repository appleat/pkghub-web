package web

import (
	"embed"
	"io/fs"
)

//go:embed ui
var uiFS embed.FS

// StaticFS 返回内嵌的静态资源文件系统（CSS/JS）。
func StaticFS() fs.FS {
	sub, err := fs.Sub(uiFS, "ui/static")
	if err != nil {
		panic(err)
	}
	return sub
}

// IndexTemplate 返回内嵌的首页模板内容。
func IndexTemplate() []byte {
	b, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		panic(err)
	}
	return b
}
