package parser

import (
	"path"
	"sort"
	"strings"
	"testing"

	"ripplegraph/internal/graph"
	"ripplegraph/internal/testutil"
)

// goldenModuleGraph は ModuleGraph をゴールデンファイル用に JSON 化しやすい形にしたもの。
type goldenModuleGraph struct {
	HasBackend  bool               `json:"hasBackend"`
	HasProvider bool               `json:"hasProvider"`
	Nodes       []Node             `json:"nodes"`
	Edges       []graph.Edge       `json:"edges"`
	Calls       []goldenCall       `json:"calls"`
	Diags       []graph.Diagnostic `json:"diags"`
}

type goldenCall struct {
	Name    string              `json:"name"`
	Source  string              `json:"source"`
	Inputs  map[string][]string `json:"inputs"`
	Count   bool                `json:"count"`
	ForEach bool                `json:"forEach"`
	File    string              `json:"file"`
	Line    int                 `json:"line"`
}

func toGolden(mg *ModuleGraph) goldenModuleGraph {
	g := goldenModuleGraph{
		HasBackend: mg.HasBackend, HasProvider: mg.HasProvider,
		Nodes: mg.Nodes, Edges: mg.Edges, Diags: mg.Diags,
		Calls: []goldenCall{},
	}
	for _, c := range mg.Calls {
		gc := goldenCall{Name: c.Name, Source: c.Source, Inputs: map[string][]string{}, Count: c.Count, ForEach: c.ForEach, File: c.File, Line: c.Line}
		for k, ts := range c.Inputs {
			ss := []string{}
			for _, t := range ts {
				ss = append(ss, TraversalString(t))
			}
			gc.Inputs[k] = ss
		}
		g.Calls = append(g.Calls, gc)
	}
	return g
}

// groupByDir はファイル群をディレクトリごとに分ける。
func groupByDir(files map[string][]byte) map[string]map[string][]byte {
	dirs := map[string]map[string][]byte{}
	for p, b := range files {
		if !strings.HasSuffix(p, ".tf") {
			continue
		}
		d := path.Dir(p)
		if dirs[d] == nil {
			dirs[d] = map[string][]byte{}
		}
		dirs[d][p] = b
	}
	return dirs
}

func TestParseModuleGolden(t *testing.T) {
	for _, fixture := range []string{"basic", "cycle"} {
		files := testutil.ReadTree(t, testutil.TestdataDir(fixture))
		dirs := groupByDir(files)
		names := make([]string, 0, len(dirs))
		for d := range dirs {
			names = append(names, d)
		}
		sort.Strings(names)
		for _, d := range names {
			t.Run(fixture+"/"+d, func(t *testing.T) {
				mg := ParseModule(dirs[d])
				testutil.AssertGolden(t, "parser/"+fixture+"/"+strings.ReplaceAll(d, "/", "__"), toGolden(mg))

				// 同じディレクトリは何度呼ばれても同じ結果を返すこと
				again := ParseModule(dirs[d])
				testutil.AssertGolden(t, "parser/"+fixture+"/"+strings.ReplaceAll(d, "/", "__"), toGolden(again))
			})
		}
	}
}

func findEdge(mg *ModuleGraph, from, to string) []graph.Edge {
	var out []graph.Edge
	for _, e := range mg.Edges {
		if e.From == from && e.To == to {
			out = append(out, e)
		}
	}
	return out
}

func hasDiag(mg *ModuleGraph, code graph.DiagCode, substr string) bool {
	for _, d := range mg.Diags {
		if d.Code == code && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

// ゴールデンファイルだけでは意図が読み取りにくい、取りこぼしやすいケースを個別に確認する。
func TestParseModuleCases(t *testing.T) {
	files := testutil.ReadTree(t, testutil.TestdataDir("basic"))
	dirs := groupByDir(files)
	dev := ParseModule(dirs["envs/dev/network"])

	t.Run("dynamic のイテレータは参照にならない", func(t *testing.T) {
		for _, d := range dev.Diags {
			if strings.Contains(d.Message, "ingress.value") || strings.Contains(d.Message, "rule.value") {
				t.Errorf("イテレータが未解決の参照として扱われています: %s", d.Message)
			}
		}
		if es := findEdge(dev, "aws_security_group.nat", "var.ingress_rules"); len(es) != 1 || es[0].Attr != "ingress.for_each" {
			t.Errorf("dynamic の for_each の参照: %+v", es)
		}
		if es := findEdge(dev, "aws_security_group.nat", "local.egress_cidrs"); len(es) != 1 || es[0].Attr != "egress.cidr_blocks" {
			t.Errorf("iterator 指定の dynamic の content の参照: %+v", es)
		}
	})

	t.Run("ignore_changes は参照にならない", func(t *testing.T) {
		if hasDiag(dev, graph.DiagCodeUnresolvedReference, "tags") {
			t.Error("ignore_changes の属性名が参照として扱われています")
		}
	})

	t.Run("depends_on と replace_triggered_by", func(t *testing.T) {
		es := findEdge(dev, "aws_eip.nat", "module.vpc")
		if len(es) != 1 || es[0].Kind != graph.EdgeKindDependsOn {
			t.Errorf("depends_on: %+v", es)
		}
		var kinds []string
		for _, e := range findEdge(dev, "aws_eip.nat", "aws_instance.nat") {
			kinds = append(kinds, string(e.Kind)+":"+e.Attr)
		}
		want := "reference:count,reference:instance,replace_triggered_by:lifecycle.replace_triggered_by"
		if strings.Join(kinds, ",") != want {
			t.Errorf("aws_eip.nat → aws_instance.nat: got %v, want %s", kinds, want)
		}
	})

	t.Run("provider 指定は参照にならない", func(t *testing.T) {
		if hasDiag(dev, graph.DiagCodeUnresolvedReference, "aws.west") {
			t.Error("provider / providers が参照として扱われています")
		}
	})

	t.Run("モジュール出力の参照は Port を持つ", func(t *testing.T) {
		es := findEdge(dev, "aws_instance.nat", "module.vpc")
		if len(es) != 1 || es[0].Port != "public_subnet_ids" || es[0].Attr != "subnet_id" {
			t.Errorf("got %+v", es)
		}
	})

	t.Run("モジュールの入力", func(t *testing.T) {
		var call *ModuleCall
		for i := range dev.Calls {
			if dev.Calls[i].Name == "vpc" {
				call = &dev.Calls[i]
			}
		}
		if call == nil || call.Source != "../../../modules/vpc" {
			t.Fatalf("module.vpc の呼び出し: %+v", call)
		}
		if got := call.Inputs["cidr"]; len(got) != 1 || TraversalString(got[0]) != "var.vpc_cidr" {
			t.Errorf("cidr の入力: %v", got)
		}
		if _, ok := call.Inputs["providers"]; ok {
			t.Error("providers が入力に含まれています")
		}
	})

	t.Run("未解決の参照", func(t *testing.T) {
		if !hasDiag(dev, graph.DiagCodeUnresolvedReference, "var.zone_id") || !hasDiag(dev, graph.DiagCodeUnresolvedReference, "aws_instance.bastion") {
			t.Errorf("未解決の参照の診断がありません: %+v", dev.Diags)
		}
	})

	t.Run("構文エラーのファイルがあっても他のファイルの結果は返る", func(t *testing.T) {
		if !hasDiag(dev, graph.DiagCodeParseError, "") {
			t.Error("parse_error がありません")
		}
		if len(findEdge(dev, "aws_instance.nat", "data.aws_ami.nat")) != 1 {
			t.Error("他のファイルの結果が返っていません")
		}
	})

	t.Run("override ファイル", func(t *testing.T) {
		if !hasDiag(dev, graph.DiagCodeUnsupportedOverride, "") {
			t.Error("unsupported_override がありません")
		}
	})

	t.Run("for 式の中の参照", func(t *testing.T) {
		if len(findEdge(dev, "local.azs", "data.aws_availability_zones.available")) != 1 || len(findEdge(dev, "local.azs", "var.excluded_az")) != 1 {
			t.Error("for 式の中の参照が取れていません")
		}
	})

	t.Run("インデックス付きのモジュール参照", func(t *testing.T) {
		app := ParseModule(dirs["envs/prd/app"])
		es := findEdge(app, "output.a_url", "module.app")
		if len(es) != 1 || es[0].Port != "url" {
			t.Errorf("got %+v", es)
		}
	})
}
