package source

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"ripplegraph/internal/graph"
)

// LocalGitSource はローカルの git リポジトリから、チェックアウトせずに任意の ref のツリーを読む。
// git リポジトリでないディレクトリも開けるが、その場合はワーキングツリーだけを読める。
type LocalGitSource struct {
	dir  string
	repo *git.Repository // git 管理外なら nil
}

// NewLocalGit は dir を開く。
func NewLocalGit(dir string) (*LocalGitSource, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s はディレクトリではありません", dir)
	}
	s := &LocalGitSource{dir: abs}
	r, err := git.PlainOpen(abs)
	switch {
	case err == nil:
		s.repo = r
	case errors.Is(err, git.ErrRepositoryNotExists):
	default:
		return nil, err
	}
	return s, nil
}

// Dir はリポジトリのディレクトリを返す。
func (s *LocalGitSource) Dir() string { return s.dir }

// Resolve は ref をコミット SHA に解決する。空の ref は WorktreeSHA を返す。
func (s *LocalGitSource) Resolve(ref string) (string, error) {
	if ref == "" || ref == WorktreeSHA {
		return WorktreeSHA, nil
	}
	if s.repo == nil {
		return "", fmt.Errorf("%w: %s（git リポジトリではありません）", ErrRefNotFound, ref)
	}
	h, err := s.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrRefNotFound, ref)
	}
	if _, err := s.repo.CommitObject(*h); err != nil {
		return "", fmt.Errorf("%w: %s（コミットではありません）", ErrRefNotFound, ref)
	}
	return h.String(), nil
}

// Files は SHA 時点のファイル群を返す。WorktreeSHA の場合はワーキングツリーを読む。
func (s *LocalGitSource) Files(sha string) (map[string][]byte, error) {
	if sha == WorktreeSHA {
		return s.worktreeFiles()
	}
	if s.repo == nil {
		return nil, fmt.Errorf("git リポジトリではありません")
	}
	commit, err := s.repo.CommitObject(plumbing.NewHash(sha))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrRefNotFound, sha)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	err = tree.Files().ForEach(func(f *object.File) error {
		if !f.Mode.IsFile() || !Wanted(f.Name) {
			return nil
		}
		r, err := f.Reader()
		if err != nil {
			return err
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		files[f.Name] = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *LocalGitSource) worktreeFiles() (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(s.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != s.dir && (d.Name() == ".git" || d.Name() == ".terraform" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(s.dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !Wanted(rel) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Refs はブランチ（ローカル・リモート）とタグの一覧を返す。
func (s *LocalGitSource) Refs() (graph.RefsResponse, error) {
	res := graph.RefsResponse{Repo: filepath.Base(s.dir), Git: s.repo != nil, Refs: []graph.Ref{}}
	if s.repo == nil {
		return res, nil
	}
	if head, err := s.repo.Head(); err == nil && head.Name().IsBranch() {
		res.Head = head.Name().Short()
	}
	iter, err := s.repo.References()
	if err != nil {
		return res, err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name()
		var kind string
		switch {
		case name.IsBranch():
			kind = "branch"
		case name.IsRemote():
			if strings.HasSuffix(name.String(), "/HEAD") {
				return nil
			}
			kind = "branch"
		case name.IsTag():
			kind = "tag"
		default:
			return nil
		}
		h := ref.Hash()
		if ref.Type() == plumbing.SymbolicReference {
			resolved, err := s.repo.Reference(name, true)
			if err != nil {
				return nil
			}
			h = resolved.Hash()
		}
		if kind == "tag" {
			// 注釈付きタグはコミットまでたどる
			if t, err := s.repo.TagObject(h); err == nil {
				if c, err := t.Commit(); err == nil {
					h = c.Hash
				}
			}
		}
		res.Refs = append(res.Refs, graph.Ref{Name: name.Short(), Kind: kind, SHA: h.String()})
		return nil
	})
	sort.Slice(res.Refs, func(i, j int) bool {
		a, b := res.Refs[i], res.Refs[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind // branch → tag
		}
		return a.Name < b.Name
	})
	return res, err
}
