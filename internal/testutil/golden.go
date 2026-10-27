// Package testutil はゴールデンテスト用のヘルパー。
//
// 期待値の更新は `go test ./... -update` で行う。テストを持つパッケージは
// -update フラグを受け付けるために必ずこのパッケージを import すること。
package testutil

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "ゴールデンファイルを更新する")

// RepoRoot は ripplegraph リポジトリのルートディレクトリを返す。
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestdataDir は testdata/<name> の絶対パスを返す。
func TestdataDir(name string) string {
	return filepath.Join(RepoRoot(), "testdata", name)
}

// ReadTree は dir 以下の .tf / .tfvars / ripplegraph.yaml を「dir からの相対パス → 内容」で読む。
func ReadTree(t testing.TB, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, ".tf") || strings.HasSuffix(rel, ".tfvars") || rel == "ripplegraph.yaml" {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[rel] = b
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// AssertGolden は v を JSON にしてゴールデンファイル（testdata/golden/<name>.json）と比較する。
func AssertGolden(t testing.TB, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(RepoRoot(), "testdata", "golden", filepath.FromSlash(name)+".json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ゴールデンファイルを読めません（-update で生成してください）: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s がゴールデンファイルと一致しません（意図した変更なら -update で更新してください）\n%s", name, diffLines(string(want), string(got)))
	}
}

// diffLines は最初に食い違った付近の行を示す簡易 diff。
func diffLines(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d", i+1) + ":\n  want: " + wl + "\n  got:  " + gl
		}
	}
	return ""
}
