// Package parser は1つのモジュールディレクトリの .tf ファイルを解析し、
// モジュール内のノードと依存（ModuleGraph）を返す。
//
// ファイルシステムやルート・モジュール展開の文脈には依存しない純粋なロジックとする。
// アドレスはモジュール内のローカルアドレス（aws_subnet.private、var.cidr など）で、
// ルートパスやモジュールのプレフィックスは repo パッケージが付ける。
package parser

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"ripplegraph/internal/graph"
)

// Node はモジュール内のノード（ローカルアドレス）。
type Node struct {
	Kind    graph.NodeKind  `json:"kind"`
	Address string          `json:"address"`
	Type    string          `json:"type,omitempty"`
	File    string          `json:"file"`
	Line    int             `json:"line"`
	Meta    *graph.NodeMeta `json:"meta,omitempty"`
}

// ModuleCall は module ブロック。
type ModuleCall struct {
	Name   string
	Source string
	// 入力名 → その式に含まれる参照（"cidr" → [var.vpc_cidr]）
	Inputs  map[string][]hcl.Traversal
	Count   bool
	ForEach bool
	File    string
	Line    int
}

// ModuleGraph は1ディレクトリ分の解析結果。
type ModuleGraph struct {
	Nodes []Node             // ローカルアドレスのノード（Address 順）
	Edges []graph.Edge       // モジュール内で解決できた依存（From / To はローカルアドレス）
	Calls []ModuleCall       // module ブロック（Name 順）
	Diags []graph.Diagnostic // パースエラー、未解決の参照、未対応構文など

	// HasBackend は terraform { backend "..." {} }（または cloud {}）を含むかどうか。
	HasBackend bool
	// HasProvider は provider ブロックを含むかどうか。
	HasProvider bool
}

// ParseModule は1ディレクトリ分の .tf ファイル（ファイル名 → 内容）を解析する。
// エラーがあっても処理を止めず、構文エラーのあるファイルは診断情報に記録して読み飛ばす。
func ParseModule(files map[string][]byte) *ModuleGraph {
	p := &moduleParser{
		mg:    &ModuleGraph{},
		nodes: map[string]*Node{},
	}

	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasSuffix(name, ".tf") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var bodies []*hclsyntax.Body
	for _, name := range names {
		if isOverrideFile(name) {
			p.diag(graph.SeverityWarning, graph.DiagCodeUnsupportedOverride,
				"override ファイルのマージには未対応のため、内容を無視しました", name, 0)
			continue
		}
		f, diags := hclsyntax.ParseConfig(files[name], name, hcl.InitialPos)
		if diags.HasErrors() {
			for _, d := range diags {
				if d.Severity != hcl.DiagError {
					continue
				}
				line := 0
				if d.Subject != nil {
					line = d.Subject.Start.Line
				}
				p.diag(graph.SeverityError, graph.DiagCodeParseError, d.Summary+": "+d.Detail, name, line)
			}
			continue
		}
		bodies = append(bodies, f.Body.(*hclsyntax.Body))
	}

	// 1パス目: ノードを集める（参照の解決先が後のファイルにあってもよいように）
	for _, body := range bodies {
		for _, blk := range body.Blocks {
			p.declare(blk)
		}
	}
	// 2パス目: 参照を集めてエッジにする
	for _, body := range bodies {
		for _, blk := range body.Blocks {
			p.collect(blk)
		}
	}

	p.finish()
	return p.mg
}

func isOverrideFile(name string) bool {
	base := path.Base(name)
	return base == "override.tf" || strings.HasSuffix(base, "_override.tf")
}

type moduleParser struct {
	mg    *ModuleGraph
	nodes map[string]*Node
	// declare で採用したブロック（重複定義を除く）
	declared map[*hclsyntax.Block]bool
	calls    map[string]*ModuleCall
}

func (p *moduleParser) diag(sev graph.Severity, code graph.DiagCode, msg, file string, line int) {
	p.mg.Diags = append(p.mg.Diags, graph.Diagnostic{
		Severity: sev, Code: code, Message: msg, File: file, Line: line,
	})
}

func blockFile(blk *hclsyntax.Block) (string, int) {
	r := blk.DefRange()
	return r.Filename, r.Start.Line
}

func (p *moduleParser) addNode(blk *hclsyntax.Block, n Node) bool {
	if prev, ok := p.nodes[n.Address]; ok {
		p.diag(graph.SeverityError, graph.DiagCodeDuplicateBlock,
			fmt.Sprintf("%s が重複して定義されています（最初の定義: %s:%d）", n.Address, prev.File, prev.Line),
			n.File, n.Line)
		return false
	}
	p.nodes[n.Address] = &n
	return true
}

func hasAttr(body *hclsyntax.Body, name string) bool {
	_, ok := body.Attributes[name]
	return ok
}

func metaOf(body *hclsyntax.Body) *graph.NodeMeta {
	m := &graph.NodeMeta{Count: hasAttr(body, "count"), ForEach: hasAttr(body, "for_each")}
	if !m.Count && !m.ForEach {
		return nil
	}
	return m
}

// declare はトップレベルのブロックからノードを作る。
func (p *moduleParser) declare(blk *hclsyntax.Block) {
	if p.declared == nil {
		p.declared = map[*hclsyntax.Block]bool{}
		p.calls = map[string]*ModuleCall{}
	}
	file, line := blockFile(blk)
	switch blk.Type {
	case "resource", "data":
		if len(blk.Labels) != 2 {
			return
		}
		kind, addr := graph.NodeKindResource, blk.Labels[0]+"."+blk.Labels[1]
		if blk.Type == "data" {
			kind, addr = graph.NodeKindData, "data."+addr
		}
		if p.addNode(blk, Node{Kind: kind, Address: addr, Type: blk.Labels[0], File: file, Line: line, Meta: metaOf(blk.Body)}) {
			p.declared[blk] = true
		}
	case "module":
		if len(blk.Labels) != 1 {
			return
		}
		name := blk.Labels[0]
		if !p.addNode(blk, Node{Kind: graph.NodeKindModule, Address: "module." + name, File: file, Line: line, Meta: metaOf(blk.Body)}) {
			return
		}
		p.declared[blk] = true
		call := &ModuleCall{
			Name:    name,
			Source:  literalString(blk.Body.Attributes["source"]),
			Inputs:  map[string][]hcl.Traversal{},
			Count:   hasAttr(blk.Body, "count"),
			ForEach: hasAttr(blk.Body, "for_each"),
			File:    file,
			Line:    line,
		}
		p.calls[name] = call
	case "variable", "output":
		if len(blk.Labels) != 1 {
			return
		}
		kind, addr := graph.NodeKindVariable, "var."+blk.Labels[0]
		if blk.Type == "output" {
			kind, addr = graph.NodeKindOutput, "output."+blk.Labels[0]
		}
		if p.addNode(blk, Node{Kind: kind, Address: addr, File: file, Line: line}) {
			p.declared[blk] = true
		}
	case "locals":
		p.declared[blk] = true
		for _, attr := range sortedAttrs(blk.Body) {
			p.addNode(blk, Node{Kind: graph.NodeKindLocal, Address: "local." + attr.Name, File: attr.SrcRange.Filename, Line: attr.SrcRange.Start.Line})
		}
	case "terraform":
		for _, b := range blk.Body.Blocks {
			if b.Type == "backend" || b.Type == "cloud" {
				p.mg.HasBackend = true
			}
		}
	case "provider":
		p.mg.HasProvider = true
	}
	// moved / import / check などは無視する
}

func literalString(attr *hclsyntax.Attribute) string {
	if attr == nil {
		return ""
	}
	v, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || !v.Type().Equals(ctyString) || v.IsNull() || !v.IsKnown() {
		return ""
	}
	return v.AsString()
}

func sortedAttrs(body *hclsyntax.Body) []*hclsyntax.Attribute {
	attrs := make([]*hclsyntax.Attribute, 0, len(body.Attributes))
	for _, a := range body.Attributes {
		attrs = append(attrs, a)
	}
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name < attrs[j].Name })
	return attrs
}

// ref はブロック内で見つかった1つの参照。
type ref struct {
	trav hcl.Traversal
	kind graph.EdgeKind
	attr string
}

// collect はトップレベルのブロックの参照を集めてエッジにする。
func (p *moduleParser) collect(blk *hclsyntax.Block) {
	if !p.declared[blk] {
		return
	}
	switch blk.Type {
	case "resource", "data":
		from := blk.Labels[0] + "." + blk.Labels[1]
		if blk.Type == "data" {
			from = "data." + from
		}
		var refs []ref
		walkBody(blk.Body, nil, nil, true, &refs)
		p.addRefs(from, refs)
	case "module":
		from := "module." + blk.Labels[0]
		call := p.calls[blk.Labels[0]]
		var refs []ref
		for _, attr := range sortedAttrs(blk.Body) {
			switch attr.Name {
			case "source", "version", "providers", "provider":
				continue
			case "depends_on":
				refs = append(refs, dependsOnRefs(attr)...)
			case "count", "for_each":
				refs = append(refs, exprRefs(attr.Expr, nil, graph.EdgeKindReference, attr.Name)...)
			default:
				rs := exprRefs(attr.Expr, nil, graph.EdgeKindReference, attr.Name)
				travs := make([]hcl.Traversal, 0, len(rs))
				for _, r := range rs {
					travs = append(travs, r.trav)
				}
				call.Inputs[attr.Name] = travs
				refs = append(refs, rs...)
			}
		}
		// module ブロックに書ける入れ子ブロックは lifecycle 相当のものだけだが、念のためたどる
		for _, b := range blk.Body.Blocks {
			walkBlock(b, nil, nil, false, &refs)
		}
		p.addRefs(from, refs)
	case "variable":
		from := "var." + blk.Labels[0]
		var refs []ref
		for _, attr := range sortedAttrs(blk.Body) {
			if attr.Name == "type" { // 型式（list(string) など）は参照ではない
				continue
			}
			refs = append(refs, exprRefs(attr.Expr, nil, graph.EdgeKindReference, attr.Name)...)
		}
		for _, b := range blk.Body.Blocks {
			walkBlock(b, nil, nil, false, &refs)
		}
		p.addRefs(from, refs)
	case "output":
		var refs []ref
		walkBody(blk.Body, nil, nil, false, &refs)
		p.addRefs("output."+blk.Labels[0], refs)
	case "locals":
		for _, attr := range sortedAttrs(blk.Body) {
			from := "local." + attr.Name
			if p.nodes[from] == nil || p.nodes[from].File != attr.SrcRange.Filename || p.nodes[from].Line != attr.SrcRange.Start.Line {
				continue // 重複定義の2つ目以降
			}
			p.addRefs(from, exprRefs(attr.Expr, nil, graph.EdgeKindReference, ""))
		}
	}
}

func joinPath(prefix []string, name string) string {
	if len(prefix) == 0 {
		return name
	}
	return strings.Join(prefix, ".") + "." + name
}

// walkBody はブロックの本体を再帰的にたどって参照を集める。
// resourceLike が true のとき、provider 属性を除外する（resource / data）。
func walkBody(body *hclsyntax.Body, prefix []string, exclude map[string]bool, resourceLike bool, refs *[]ref) {
	for _, attr := range sortedAttrs(body) {
		name := joinPath(prefix, attr.Name)
		switch {
		case len(prefix) == 0 && attr.Name == "depends_on":
			*refs = append(*refs, dependsOnRefs(attr)...)
			continue
		case len(prefix) == 0 && resourceLike && attr.Name == "provider":
			continue
		}
		*refs = append(*refs, exprRefs(attr.Expr, exclude, graph.EdgeKindReference, name)...)
	}
	for _, b := range body.Blocks {
		walkBlock(b, prefix, exclude, resourceLike, refs)
	}
}

func walkBlock(b *hclsyntax.Block, prefix []string, exclude map[string]bool, resourceLike bool, refs *[]ref) {
	switch {
	case b.Type == "lifecycle" && len(prefix) == 0:
		p := []string{"lifecycle"}
		for _, attr := range sortedAttrs(b.Body) {
			switch attr.Name {
			case "ignore_changes": // 属性名の並びであり参照ではない
			case "replace_triggered_by":
				*refs = append(*refs, exprRefs(attr.Expr, exclude, graph.EdgeKindReplaceTriggeredBy, joinPath(p, attr.Name))...)
			default:
				*refs = append(*refs, exprRefs(attr.Expr, exclude, graph.EdgeKindReference, joinPath(p, attr.Name))...)
			}
		}
		for _, nb := range b.Body.Blocks {
			walkBlock(nb, p, exclude, resourceLike, refs)
		}
	case b.Type == "dynamic" && len(b.Labels) == 1:
		label := b.Labels[0]
		p := append(append([]string{}, prefix...), label)
		iter := label
		if a, ok := b.Body.Attributes["iterator"]; ok {
			if t, diags := hcl.AbsTraversalForExpr(a.Expr); !diags.HasErrors() && len(t) == 1 {
				iter = t.RootName()
			}
		}
		inner := map[string]bool{iter: true}
		for k := range exclude {
			inner[k] = true
		}
		for _, attr := range sortedAttrs(b.Body) {
			switch attr.Name {
			case "iterator":
			case "for_each": // 除外の外側で評価する
				*refs = append(*refs, exprRefs(attr.Expr, exclude, graph.EdgeKindReference, joinPath(p, attr.Name))...)
			default: // labels など
				*refs = append(*refs, exprRefs(attr.Expr, inner, graph.EdgeKindReference, joinPath(p, attr.Name))...)
			}
		}
		for _, nb := range b.Body.Blocks {
			if nb.Type == "content" {
				walkBody(nb.Body, p, inner, false, refs)
			} else {
				walkBlock(nb, p, inner, false, refs)
			}
		}
	default:
		walkBody(b.Body, append(append([]string{}, prefix...), b.Type), exclude, false, refs)
	}
}

func exprRefs(expr hclsyntax.Expression, exclude map[string]bool, kind graph.EdgeKind, attr string) []ref {
	var refs []ref
	for _, t := range expr.Variables() {
		if exclude[t.RootName()] {
			continue
		}
		refs = append(refs, ref{trav: t, kind: kind, attr: attr})
	}
	return refs
}

func dependsOnRefs(attr *hclsyntax.Attribute) []ref {
	exprs, diags := hcl.ExprList(attr.Expr)
	if diags.HasErrors() {
		return exprRefs(attr.Expr, nil, graph.EdgeKindDependsOn, attr.Name)
	}
	var refs []ref
	for _, e := range exprs {
		t, diags := hcl.AbsTraversalForExpr(e)
		if diags.HasErrors() {
			continue
		}
		refs = append(refs, ref{trav: t, kind: graph.EdgeKindDependsOn, attr: attr.Name})
	}
	return refs
}

// resolve は参照をモジュール内のアドレスに解決する。
// ignore が true の参照（count、each など）は無視する。
func resolve(t hcl.Traversal) (addr, port string, ignore bool) {
	root := t.RootName()
	var names []string
	for _, step := range t[1:] {
		if a, ok := step.(hcl.TraverseAttr); ok {
			names = append(names, a.Name)
		}
		// インデックス（[0]、["a"]）やスプラットは取り除く
	}
	switch root {
	case "count", "each", "self", "path", "terraform":
		return "", "", true
	case "var", "local":
		if len(names) < 1 {
			return "", "", false
		}
		return root + "." + names[0], "", false
	case "data":
		if len(names) < 2 {
			return "", "", false
		}
		return "data." + names[0] + "." + names[1], "", false
	case "module":
		if len(names) < 1 {
			return "", "", false
		}
		if len(names) >= 2 {
			port = names[1]
		}
		return "module." + names[0], port, false
	default:
		if len(names) < 1 {
			return "", "", false
		}
		return root + "." + names[0], "", false
	}
}

func (p *moduleParser) addRefs(from string, refs []ref) {
	for _, r := range refs {
		rng := r.trav.SourceRange()
		to, port, ignore := resolve(r.trav)
		if ignore {
			continue
		}
		if to == "" || p.nodes[to] == nil {
			p.diag(graph.SeverityWarning, graph.DiagCodeUnresolvedReference,
				fmt.Sprintf("%s の参照 %s を解決できません", from, TraversalString(r.trav)),
				rng.Filename, rng.Start.Line)
			continue
		}
		if to == from {
			continue
		}
		p.mg.Edges = append(p.mg.Edges, graph.Edge{
			From: from, To: to, Kind: r.kind, Attr: r.attr, Port: port,
			File: rng.Filename, Line: rng.Start.Line,
		})
	}
}

func (p *moduleParser) finish() {
	mg := p.mg
	for _, n := range p.nodes {
		mg.Nodes = append(mg.Nodes, *n)
	}
	sort.Slice(mg.Nodes, func(i, j int) bool { return mg.Nodes[i].Address < mg.Nodes[j].Address })

	for _, c := range p.calls {
		mg.Calls = append(mg.Calls, *c)
	}
	sort.Slice(mg.Calls, func(i, j int) bool { return mg.Calls[i].Name < mg.Calls[j].Name })

	SortEdges(mg.Edges)
	// 同じ参照元・参照先・種別・属性・ポートの重複エッジは1本にまとめる（最初の出現を残す）
	uniq := mg.Edges[:0]
	seen := map[[5]string]bool{}
	for _, e := range mg.Edges {
		k := [5]string{e.From, e.To, string(e.Kind), e.Attr, e.Port}
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, e)
	}
	mg.Edges = uniq

	SortDiags(mg.Diags)
}

// SortEdges はエッジを決定的な順序に並べる。
func SortEdges(edges []graph.Edge) {
	sort.SliceStable(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Attr != b.Attr {
			return a.Attr < b.Attr
		}
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}

// SortDiags は診断情報を決定的な順序に並べる。
func SortDiags(diags []graph.Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
}

// TraversalString は参照を Terraform の記法で文字列にする（例: module.app["a"].url）。
func TraversalString(t hcl.Traversal) string {
	var sb strings.Builder
	for _, step := range t {
		switch s := step.(type) {
		case hcl.TraverseRoot:
			sb.WriteString(s.Name)
		case hcl.TraverseAttr:
			sb.WriteString("." + s.Name)
		case hcl.TraverseIndex:
			sb.WriteString("[" + indexString(s) + "]")
		case hcl.TraverseSplat:
			sb.WriteString("[*]")
		}
	}
	return sb.String()
}
