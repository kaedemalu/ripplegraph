package source

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	_ "ripplegraph/internal/testutil" // -update フラグを受け付けるため
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalGitSource(t *testing.T) {
	dir := t.TempDir()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "envs/dev/main.tf", `resource "a" "b" {}`)
	writeFile(t, dir, "envs/dev/terraform.tfvars", `x = 1`)
	writeFile(t, dir, "envs/dev/.terraform/modules/m/main.tf", `resource "x" "y" {}`)
	writeFile(t, dir, "ripplegraph.yaml", `pathPattern: "envs/{env}"`)
	writeFile(t, dir, "README.md", `readme`)

	wt, _ := r.Worktree()
	for _, p := range []string{"envs/dev/main.tf", "envs/dev/terraform.tfvars", "ripplegraph.yaml", "README.md"} {
		if _, err := wt.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Unix(0, 0)}
	commit, err := wt.Commit("init", &git.CommitOptions{Author: sig})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateTag("v1", commit, &git.CreateTagOptions{Tagger: sig, Message: "v1"}); err != nil {
		t.Fatal(err)
	}

	// コミット後のワーキングツリーの変更
	writeFile(t, dir, "envs/dev/new.tf", `resource "c" "d" {}`)

	s, err := NewLocalGit(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range []string{"master", "v1", commit.String(), commit.String()[:8]} {
		sha, err := s.Resolve(ref)
		if err != nil || sha != commit.String() {
			t.Errorf("Resolve(%q) = %s, %v", ref, sha, err)
		}
	}
	if _, err := s.Resolve("no-such-branch"); !errors.Is(err, ErrRefNotFound) {
		t.Errorf("存在しない ref: %v", err)
	}

	files, err := s.Files(commit.String())
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, files, "envs/dev/main.tf", "envs/dev/terraform.tfvars", "ripplegraph.yaml")

	sha, _ := s.Resolve("")
	if sha != WorktreeSHA {
		t.Fatalf("Resolve(\"\") = %s", sha)
	}
	files, err = s.Files(sha)
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, files, "envs/dev/main.tf", "envs/dev/new.tf", "envs/dev/terraform.tfvars", "ripplegraph.yaml")

	refs, err := s.Refs()
	if err != nil {
		t.Fatal(err)
	}
	if refs.Head != "master" || len(refs.Refs) != 2 || refs.Refs[1].Kind != "tag" || refs.Refs[1].SHA != commit.String() {
		t.Errorf("refs = %+v", refs)
	}
}

func TestNonGitDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `resource "a" "b" {}`)
	s, err := NewLocalGit(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve("main"); err == nil {
		t.Error("git 管理外で ref が解決できてしまいます")
	}
	files, err := s.Files(WorktreeSHA)
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, files, "main.tf")
	refs, _ := s.Refs()
	if refs.Git {
		t.Error("Git = true")
	}
}

func assertKeys(t *testing.T, files map[string][]byte, want ...string) {
	t.Helper()
	if len(files) != len(want) {
		t.Errorf("files = %v, want %v", keys(files), want)
		return
	}
	for _, k := range want {
		if _, ok := files[k]; !ok {
			t.Errorf("files = %v, want %v", keys(files), want)
			return
		}
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
