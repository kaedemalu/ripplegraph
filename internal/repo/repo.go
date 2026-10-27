// Package repo はリポジトリ全体のファイル群から、ルート判定とモジュール展開を行い、
// RepoGraph / RootGraph を組み立てる。
//
// モジュール境界の接続方式（粗い方式／正確な方式）の判断はこのパッケージだけで行う。
// ステップ1は粗い方式（connectCoarse）で、モジュール境界を module ノードで止める。
package repo

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"ripplegraph/internal/graph"
	"ripplegraph/internal/parser"
)

// Repo は特定の SHA 時点のリポジトリ。ディレクトリ単位のパース結果と
// ルートごとの RootGraph を遅延計算してキャッシュする。並行に呼び出してよい。
type Repo struct {
	sha         string
	dirs        map[string]map[string][]byte // ディレクトリ → .tf ファイル
	config      Config
	configDiags []graph.Diagnostic

	mu         sync.Mutex
	parsed     map[string]*parser.ModuleGraph
	roots      []string
	rootSet    map[string]bool
	rootGraphs map[string]*graph.RootGraph
	repoGraph  *graph.RepoGraph
}

// Load はファイル群（リポジトリルートからの相対パス → 内容）から Repo を作る。
// files には .tf / .tfvars と、存在すれば ripplegraph.yaml を含める。
func Load(sha string, files map[string][]byte) *Repo {
	r := &Repo{
		sha:        sha,
		dirs:       map[string]map[string][]byte{},
		parsed:     map[string]*parser.ModuleGraph{},
		rootGraphs: map[string]*graph.RootGraph{},
	}
	r.config, r.configDiags = parseConfig(files[ConfigFile])
	for p, b := range files {
		if !strings.HasSuffix(p, ".tf") || strings.Contains("/"+p, "/.terraform/") {
			continue
		}
		d := path.Dir(p)
		if r.dirs[d] == nil {
			r.dirs[d] = map[string][]byte{}
		}
		r.dirs[d][p] = b
	}
	r.detectRoots()
	return r
}

// SHA はこの Repo の SHA を返す。
func (r *Repo) SHA() string { return r.sha }

// Roots はルートのパスを昇順で返す。
func (r *Repo) Roots() []string { return append([]string{}, r.roots...) }

// IsRoot は path がルートかどうかを返す。
func (r *Repo) IsRoot(p string) bool { return r.rootSet[p] }

func (r *Repo) sortedDirs() []string {
	dirs := make([]string, 0, len(r.dirs))
	for d := range r.dirs {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// module はディレクトリのパース結果を返す（キャッシュ済み）。
func (r *Repo) module(dir string) *parser.ModuleGraph {
	r.mu.Lock()
	mg, ok := r.parsed[dir]
	r.mu.Unlock()
	if ok {
		return mg
	}
	mg = parser.ParseModule(r.dirs[dir])
	r.mu.Lock()
	r.parsed[dir] = mg
	r.mu.Unlock()
	return mg
}

// isLocalSource は相対パスのモジュールソースかどうかを判定する。
func isLocalSource(src string) bool {
	return strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../")
}

// resolveSource はモジュールソースをリポジトリルートからのディレクトリパスに解決する。
// リポジトリ外を指す場合は ok=false。
func resolveSource(dir, src string) (string, bool) {
	p := path.Join(dir, src)
	if p == ".." || strings.HasPrefix(p, "../") {
		return p, false
	}
	return p, true
}

// detectRoots はルートを判定する。
// ripplegraph.yaml の roots.include があればそれに一致するディレクトリ、なければ
// backend または provider ブロックを含み、どこからもモジュールとして参照されていないディレクトリ。
func (r *Repo) detectRoots() {
	referenced := map[string]bool{}
	for _, d := range r.sortedDirs() {
		for _, c := range r.module(d).Calls {
			if !isLocalSource(c.Source) {
				continue
			}
			if p, ok := resolveSource(d, c.Source); ok {
				referenced[p] = true
			}
		}
	}

	r.rootSet = map[string]bool{}
	for _, d := range r.sortedDirs() {
		var isRoot bool
		if len(r.config.Roots.Include) > 0 {
			for _, g := range r.config.Roots.Include {
				if matchGlob(g, d) {
					isRoot = true
					break
				}
			}
		} else {
			mg := r.module(d)
			isRoot = (mg.HasBackend || mg.HasProvider) && !referenced[d]
		}
		for _, g := range r.config.Roots.Exclude {
			if matchGlob(g, d) {
				isRoot = false
			}
		}
		if isRoot {
			r.rootSet[d] = true
			r.roots = append(r.roots, d)
		}
	}
}

// Tags はルートの env / component を pathPattern から取り出す。
func (r *Repo) Tags(root string) (env, component string) {
	tags, ok := matchPathPattern(r.config.PathPattern, root)
	if !ok {
		return "", ""
	}
	return tags["env"], tags["component"]
}

// RootGraph は1ルートのモジュール展開済みのグラフを返す。ルートでなければ ok=false。
// 返り値は共有されるので変更しないこと。
func (r *Repo) RootGraph(root string) (*graph.RootGraph, bool) {
	if !r.rootSet[root] {
		return nil, false
	}
	r.mu.Lock()
	g, ok := r.rootGraphs[root]
	r.mu.Unlock()
	if ok {
		return g, true
	}
	g = r.buildRootGraph(root)
	r.mu.Lock()
	r.rootGraphs[root] = g
	r.mu.Unlock()
	return g, true
}

type expander struct {
	r     *Repo
	root  string
	nodes []graph.Node
	edges []graph.Edge
	diags []graph.Diagnostic
}

func (r *Repo) buildRootGraph(root string) *graph.RootGraph {
	x := &expander{r: r, root: root}
	x.expand(root, "", "", []string{root})

	env, component := r.Tags(root)
	sort.Slice(x.nodes, func(i, j int) bool { return x.nodes[i].ID < x.nodes[j].ID })
	parser.SortEdges(x.edges)
	x.diags = dedupDiags(x.diags)
	parser.SortDiags(x.diags)
	return &graph.RootGraph{
		SHA: r.sha, Root: root, Env: env, Component: component,
		Nodes: nonNil(x.nodes), Edges: nonNil(x.edges), Diagnostics: nonNil(x.diags),
	}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (x *expander) id(prefix, addr string) string {
	return x.root + "::" + prefix + addr
}

// expand は dir のモジュールを prefix（"module.vpc." など）付きで展開する。
// parentID は親 module ノードの ID、stack は循環検出用の呼び出し中のディレクトリ。
func (x *expander) expand(dir, prefix, parentID string, stack []string) {
	mg := x.r.module(dir)
	x.diags = append(x.diags, mg.Diags...)

	calls := map[string]*parser.ModuleCall{}
	for i := range mg.Calls {
		calls["module."+mg.Calls[i].Name] = &mg.Calls[i]
	}

	for _, n := range mg.Nodes {
		node := graph.Node{
			ID: x.id(prefix, n.Address), Kind: n.Kind, Address: prefix + n.Address, Type: n.Type,
			ParentModule: parentID, File: n.File, Line: n.Line, Meta: n.Meta,
		}
		call := calls[n.Address]
		if call == nil {
			x.nodes = append(x.nodes, node)
			continue
		}
		if !isLocalSource(call.Source) {
			node.External = true
			x.nodes = append(x.nodes, node)
			x.diags = append(x.diags, graph.Diagnostic{
				Severity: graph.SeverityInfo, Code: graph.DiagCodeExternalModule,
				Message: fmt.Sprintf("%s のソース %q はリポジトリ外のため中身を解決しません", node.Address, call.Source),
				File:    call.File, Line: call.Line,
			})
			continue
		}
		x.nodes = append(x.nodes, node)
		target, ok := resolveSource(dir, call.Source)
		if !ok || x.r.dirs[target] == nil {
			x.diags = append(x.diags, graph.Diagnostic{
				Severity: graph.SeverityError, Code: graph.DiagCodeModuleNotFound,
				Message: fmt.Sprintf("%s のソース %q のディレクトリ（%s）に .tf ファイルがありません", node.Address, call.Source, target),
				File:    call.File, Line: call.Line,
			})
			continue
		}
		if contains(stack, target) {
			x.diags = append(x.diags, graph.Diagnostic{
				Severity: graph.SeverityError, Code: graph.DiagCodeModuleCycle,
				Message: fmt.Sprintf("%s でモジュールの循環参照を検出したため展開を打ち切りました（%s → %s）", node.Address, strings.Join(stack, " → "), target),
				File:    call.File, Line: call.Line,
			})
			continue
		}
		x.expand(target, node.Address+".", node.ID, append(append([]string{}, stack...), target))
	}

	for _, e := range mg.Edges {
		x.edges = append(x.edges, connectCoarse(e, calls, x.id(prefix, e.From), x.id(prefix, e.To)))
	}
}

// connectCoarse はモジュール境界を「粗い方式」で接続する。
//   - 外から出力を参照: X → module.N（Kind: module_io、Port: 出力名）
//   - モジュールへの入力: module.N → 入力の参照先（Kind: module_io、Attr: 入力名）
//
// 将来の「正確な方式」では、ここで Port / ModuleCall.Inputs を使って
// 内部の output.* / var.* へ付け替える。
func connectCoarse(e graph.Edge, calls map[string]*parser.ModuleCall, from, to string) graph.Edge {
	out := e
	out.From, out.To = from, to
	if e.Kind != graph.EdgeKindReference {
		return out
	}
	if calls[e.To] != nil {
		out.Kind = graph.EdgeKindModuleIO
	}
	if c := calls[e.From]; c != nil {
		if _, isInput := c.Inputs[e.Attr]; isInput {
			out.Kind = graph.EdgeKindModuleIO
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func dedupDiags(diags []graph.Diagnostic) []graph.Diagnostic {
	seen := map[graph.Diagnostic]bool{}
	out := diags[:0:0]
	for _, d := range diags {
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

// RepoGraph はリポジトリ全体のグラフを返す。返り値は共有されるので変更しないこと。
func (r *Repo) RepoGraph() *graph.RepoGraph {
	r.mu.Lock()
	g := r.repoGraph
	r.mu.Unlock()
	if g != nil {
		return g
	}
	g = r.buildRepoGraph()
	r.mu.Lock()
	r.repoGraph = g
	r.mu.Unlock()
	return g
}

func (r *Repo) buildRepoGraph() *graph.RepoGraph {
	g := &graph.RepoGraph{SHA: r.sha, Roots: []graph.RepoRoot{}, Modules: []graph.RepoModule{}, Edges: []graph.RepoEdge{}}
	diags := append([]graph.Diagnostic{}, r.configDiags...)

	for _, root := range r.roots {
		rg, _ := r.RootGraph(root)
		count := 0
		for _, n := range rg.Nodes {
			if n.Kind == graph.NodeKindResource || n.Kind == graph.NodeKindData {
				count++
			}
		}
		env, component := r.Tags(root)
		g.Roots = append(g.Roots, graph.RepoRoot{Path: root, Env: env, Component: component, ResourceCount: count})
	}

	// ルートから到達できるモジュールとモジュール呼び出しのエッジ
	modules := map[string]bool{}
	edges := map[graph.RepoEdge]bool{}
	visited := map[string]bool{}
	var visit func(dir string)
	visit = func(dir string) {
		if visited[dir] {
			return
		}
		visited[dir] = true
		mg := r.module(dir)
		for _, d := range mg.Diags {
			if d.Severity == graph.SeverityError {
				diags = append(diags, d)
			}
		}
		for _, c := range mg.Calls {
			if !isLocalSource(c.Source) {
				continue
			}
			target, ok := resolveSource(dir, c.Source)
			if !ok || r.dirs[target] == nil {
				diags = append(diags, graph.Diagnostic{
					Severity: graph.SeverityError, Code: graph.DiagCodeModuleNotFound,
					Message: fmt.Sprintf("module.%s のソース %q のディレクトリ（%s）に .tf ファイルがありません", c.Name, c.Source, target),
					File:    c.File, Line: c.Line,
				})
				continue
			}
			edges[graph.RepoEdge{From: dir, To: target, Kind: graph.RepoEdgeKindModuleCall}] = true
			if !r.rootSet[target] {
				modules[target] = true
			}
			visit(target)
		}
	}
	for _, root := range r.roots {
		visit(root)
	}

	for m := range modules {
		g.Modules = append(g.Modules, graph.RepoModule{Path: m})
	}
	sort.Slice(g.Modules, func(i, j int) bool { return g.Modules[i].Path < g.Modules[j].Path })
	for e := range edges {
		g.Edges = append(g.Edges, e)
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	diags = dedupDiags(diags)
	parser.SortDiags(diags)
	g.Diagnostics = nonNil(diags)
	return g
}

// Search は全ルートのノードからアドレスの部分一致（大文字小文字を区別しない）で検索する。
// var / local / output は対象外。limit を超えた場合 truncated=true を返す。
func (r *Repo) Search(q string, limit int) (hits []graph.SearchHit, truncated bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	hits = []graph.SearchHit{}
	if q == "" {
		return hits, false
	}
	for _, root := range r.roots {
		rg, _ := r.RootGraph(root)
		for _, n := range rg.Nodes {
			switch n.Kind {
			case graph.NodeKindResource, graph.NodeKindData, graph.NodeKindModule:
			default:
				continue
			}
			if !strings.Contains(strings.ToLower(n.Address), q) {
				continue
			}
			if len(hits) >= limit {
				return hits, true
			}
			hits = append(hits, graph.SearchHit{Root: root, NodeID: n.ID, Address: n.Address, Kind: n.Kind, Type: n.Type})
		}
	}
	return hits, false
}
