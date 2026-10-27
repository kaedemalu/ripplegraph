// Package source は特定の ref 時点のファイル群を提供する入力元を定義する。
package source

import (
	"errors"
	"path"
	"strings"

	"ripplegraph/internal/graph"
)

// WorktreeSHA はワーキングツリーを表す特別な SHA。キャッシュしない。
const WorktreeSHA = "worktree"

// ErrRefNotFound は ref を解決できなかったことを表す。
var ErrRefNotFound = errors.New("ref が見つかりません")

// Source は特定の ref 時点のファイル群を提供する。
type Source interface {
	// Resolve は ref（ブランチ名、タグ、SHA）をコミット SHA に解決する。
	// 空の ref はワーキングツリー（WorktreeSHA）を表す。
	Resolve(ref string) (sha string, err error)
	// Files は指定 SHA の .tf / .tfvars ファイル（とリポジトリ直下の ripplegraph.yaml）を
	// 「リポジトリルートからの相対パス → 内容」で返す。
	Files(sha string) (map[string][]byte, error)
}

// RefLister は UI の ref 選択用にブランチ・タグの一覧を返す。
type RefLister interface {
	Refs() (graph.RefsResponse, error)
}

// Wanted は読み込み対象のファイルかどうかを判定する。
// .tf / .tfvars（.auto.tfvars を含む）と、リポジトリ直下の ripplegraph.yaml が対象。
// .terraform/ ディレクトリの中は除外する。
func Wanted(p string) bool {
	if strings.Contains("/"+p, "/.terraform/") {
		return false
	}
	if p == "ripplegraph.yaml" {
		return true
	}
	ext := path.Ext(p)
	return ext == ".tf" || ext == ".tfvars"
}
