// Package web は Next.js の静的エクスポート（web/out）を Go バイナリに埋め込む。
//
// web/out が無いとコンパイルできないため、UI をビルドせずに go test / go vet を
// 通すときは `make web-placeholder` で空の web/out を作ること。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:out
var dist embed.FS

// FS は web/out をルートとする静的ファイルを返す。
func FS() fs.FS {
	sub, err := fs.Sub(dist, "out")
	if err != nil {
		panic(err)
	}
	return sub
}
