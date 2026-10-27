// Command ripplegraph は指定 ref の RepoGraph / RootGraph を JSON で標準出力に出す CLI。
//
//	ripplegraph -repo /path/to/infra-monorepo [-ref main] [-root envs/prd/network]
//
// ステップ3で GitLab CI のジョブから使う想定。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"ripplegraph/internal/repo"
	"ripplegraph/internal/source"
)

func main() {
	repoDir := flag.String("repo", ".", "対象の Terraform リポジトリのディレクトリ")
	ref := flag.String("ref", "", "ブランチ名・タグ・SHA（省略時はワーキングツリー）")
	root := flag.String("root", "", "指定するとそのルートの RootGraph を出力する（省略時は RepoGraph）")
	flag.Parse()

	if err := run(*repoDir, *ref, *root); err != nil {
		fmt.Fprintln(os.Stderr, "ripplegraph:", err)
		os.Exit(1)
	}
}

func run(repoDir, ref, root string) error {
	src, err := source.NewLocalGit(repoDir)
	if err != nil {
		return err
	}
	sha, err := src.Resolve(ref)
	if err != nil {
		return err
	}
	files, err := src.Files(sha)
	if err != nil {
		return err
	}
	r := repo.Load(sha, files)

	var out any = r.RepoGraph()
	if root != "" {
		rg, ok := r.RootGraph(root)
		if !ok {
			return fmt.Errorf("ルートが見つかりません: %s（ルート: %v）", root, r.Roots())
		}
		out = rg
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
