package repo

import (
	"strings"
	"testing"

	"ripplegraph/internal/graph"
	"ripplegraph/internal/testutil"
)

func loadFixture(t *testing.T, name string) *Repo {
	t.Helper()
	return Load("test-sha", testutil.ReadTree(t, testutil.TestdataDir(name)))
}

func TestRepoGolden(t *testing.T) {
	for _, fixture := range []string{"basic", "cycle"} {
		r := loadFixture(t, fixture)
		t.Run(fixture+"/repo", func(t *testing.T) {
			testutil.AssertGolden(t, "repo/"+fixture+"/_repo", r.RepoGraph())
		})
		for _, root := range r.Roots() {
			t.Run(fixture+"/"+root, func(t *testing.T) {
				rg, ok := r.RootGraph(root)
				if !ok {
					t.Fatalf("%s がルートではありません", root)
				}
				testutil.AssertGolden(t, "repo/"+fixture+"/"+strings.ReplaceAll(root, "/", "__"), rg)
			})
		}
	}
}

func TestRootDetection(t *testing.T) {
	r := loadFixture(t, "basic")
	got := strings.Join(r.Roots(), ",")
	want := "envs/dev/app,envs/dev/network,envs/prd/app,envs/prd/network,global/iam"
	if got != want {
		t.Errorf("roots = %s, want %s", got, want)
	}

	c := loadFixture(t, "cycle")
	if got := strings.Join(c.Roots(), ","); got != "root" {
		t.Errorf("cycle roots = %s", got)
	}
}

func TestRootDetectionWithConfig(t *testing.T) {
	files := testutil.ReadTree(t, testutil.TestdataDir("basic"))
	files[ConfigFile] = []byte(`
roots:
  include: ["envs/**"]
  exclude: ["envs/dev/*"]
pathPattern: "envs/{env}/{component}"
`)
	r := Load("x", files)
	if got := strings.Join(r.Roots(), ","); got != "envs/prd/app,envs/prd/network" {
		t.Errorf("roots = %s", got)
	}
}

func TestInvalidConfig(t *testing.T) {
	files := testutil.ReadTree(t, testutil.TestdataDir("basic"))
	files[ConfigFile] = []byte("roots: [")
	r := Load("x", files)
	g := r.RepoGraph()
	found := false
	for _, d := range g.Diagnostics {
		found = found || d.Code == graph.DiagCodeConfigError
	}
	if !found {
		t.Errorf("config_error がありません: %+v", g.Diagnostics)
	}
	// 既定の設定（自動検出）で続行する
	if len(r.Roots()) != 5 {
		t.Errorf("roots = %v", r.Roots())
	}
}

func TestTags(t *testing.T) {
	r := loadFixture(t, "basic")
	if env, comp := r.Tags("envs/prd/network"); env != "prd" || comp != "network" {
		t.Errorf("tags = %s, %s", env, comp)
	}
	if env, comp := r.Tags("global/iam"); env != "" || comp != "" {
		t.Errorf("pathPattern に一致しないルートにタグが付いています: %s, %s", env, comp)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pat, dir string
		want     bool
	}{
		{"envs/*/*", "envs/dev/network", true},
		{"envs/*/*", "envs/dev", false},
		{"envs/*/*", "envs/dev/network/sub", false},
		{"envs/**", "envs/dev/network/sub", true},
		{"**/network", "envs/dev/network", true},
		{"**", ".", true},
	}
	for _, c := range cases {
		if got := matchGlob(c.pat, c.dir); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v", c.pat, c.dir, got)
		}
	}
}

func nodeByID(rg *graph.RootGraph, id string) *graph.Node {
	for i := range rg.Nodes {
		if rg.Nodes[i].ID == id {
			return &rg.Nodes[i]
		}
	}
	return nil
}

func TestModuleExpansion(t *testing.T) {
	r := loadFixture(t, "basic")
	rg, _ := r.RootGraph("envs/prd/network")

	t.Run("ID は一意", func(t *testing.T) {
		seen := map[string]bool{}
		for _, n := range rg.Nodes {
			if seen[n.ID] {
				t.Errorf("ID が衝突しています: %s", n.ID)
			}
			seen[n.ID] = true
		}
	})

	t.Run("同じモジュールの複数回呼び出し・入れ子", func(t *testing.T) {
		for _, id := range []string{
			"envs/prd/network::module.vpc.aws_vpc.this",
			"envs/prd/network::module.vpc_shared.aws_vpc.this",
			"envs/prd/network::module.vpc.module.public_subnets.aws_subnet.this",
			"envs/prd/network::module.vpc.module.private_subnets.aws_subnet.this",
		} {
			if nodeByID(rg, id) == nil {
				t.Errorf("%s がありません", id)
			}
		}
		n := nodeByID(rg, "envs/prd/network::module.vpc.module.public_subnets.aws_subnet.this")
		if n.ParentModule != "envs/prd/network::module.vpc.module.public_subnets" {
			t.Errorf("parentModule = %s", n.ParentModule)
		}
		if p := nodeByID(rg, n.ParentModule); p.ParentModule != "envs/prd/network::module.vpc" {
			t.Errorf("入れ子の parentModule = %s", p.ParentModule)
		}
	})

	t.Run("粗い方式のモジュール境界", func(t *testing.T) {
		dev, _ := r.RootGraph("envs/dev/network")
		var out, in bool
		for _, e := range dev.Edges {
			if e.From == "envs/dev/network::aws_instance.nat" && e.To == "envs/dev/network::module.vpc" && e.Kind == graph.EdgeKindModuleIO && e.Port == "public_subnet_ids" {
				out = true
			}
			if e.From == "envs/dev/network::module.vpc" && e.To == "envs/dev/network::var.vpc_cidr" && e.Kind == graph.EdgeKindModuleIO && e.Attr == "cidr" {
				in = true
			}
			// depends_on は module_io にしない
			if e.From == "envs/dev/network::aws_eip.nat" && e.To == "envs/dev/network::module.vpc" && e.Kind != graph.EdgeKindDependsOn {
				t.Errorf("depends_on の種別が変わっています: %+v", e)
			}
		}
		if !out || !in {
			t.Errorf("module_io のエッジがありません（出力参照: %v、入力: %v）", out, in)
		}
	})

	t.Run("registry ソースは中身の見えないノード", func(t *testing.T) {
		dev, _ := r.RootGraph("envs/dev/network")
		n := nodeByID(dev, "envs/dev/network::module.flow_logs")
		if n == nil || !n.External {
			t.Errorf("module.flow_logs: %+v", n)
		}
	})

	t.Run("存在しないモジュール", func(t *testing.T) {
		app, _ := r.RootGraph("envs/prd/app")
		found := false
		for _, d := range app.Diagnostics {
			found = found || d.Code == graph.DiagCodeModuleNotFound
		}
		if !found {
			t.Error("module_not_found がありません")
		}
	})

	t.Run("循環参照", func(t *testing.T) {
		c := loadFixture(t, "cycle")
		rg, _ := c.RootGraph("root")
		found := false
		for _, d := range rg.Diagnostics {
			found = found || d.Code == graph.DiagCodeModuleCycle
		}
		if !found {
			t.Error("module_cycle がありません")
		}
	})
}

func TestResourceCount(t *testing.T) {
	r := loadFixture(t, "basic")
	for _, root := range r.RepoGraph().Roots {
		if root.Path == "envs/prd/network" {
			// vpc (for_each) と vpc_shared がそれぞれ aws_vpc + subnet×2、加えて peering と nat
			if root.ResourceCount != 8 {
				t.Errorf("resourceCount = %d", root.ResourceCount)
			}
		}
	}
}

func TestSearch(t *testing.T) {
	r := loadFixture(t, "basic")
	hits, truncated := r.Search("AWS_SUBNET", 100)
	if truncated || len(hits) == 0 {
		t.Fatalf("hits = %v", hits)
	}
	for _, h := range hits {
		if !strings.Contains(h.Address, "aws_subnet") {
			t.Errorf("一致しない結果: %+v", h)
		}
	}
	if _, truncated := r.Search("aws", 1); !truncated {
		t.Error("truncated になっていません")
	}
}
