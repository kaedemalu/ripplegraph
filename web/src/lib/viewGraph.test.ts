import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import type { Edge, Node, RootGraph } from "@/types/graph";
import { ancestorModules, buildViewGraph, defaultViewOptions, resourceTypes, type ViewOptions } from "./viewGraph";

const R = "r";
const n = (address: string, extra: Partial<Node> = {}): Node => {
  const kind: Node["kind"] = address.startsWith("var.")
    ? "variable"
    : address.startsWith("local.")
      ? "local"
      : address.startsWith("output.")
        ? "output"
        : address.startsWith("data.")
          ? "data"
          : address.startsWith("module.") && !address.slice(7).includes(".")
            ? "module"
            : "resource";
  return { id: `${R}::${address}`, kind, address, file: "main.tf", line: 1, ...extra };
};
const e = (from: string, to: string, extra: Partial<Edge> = {}): Edge => ({
  from: `${R}::${from}`,
  to: `${R}::${to}`,
  kind: "reference",
  file: "main.tf",
  line: 1,
  ...extra,
});
const graph = (nodes: Node[], edges: Edge[]): RootGraph => ({ sha: "x", root: R, nodes, edges, diagnostics: [] });
const opts = (o: Partial<ViewOptions> = {}): ViewOptions => ({ ...defaultViewOptions, ...o });
const ids = (xs: { id: string }[]) => xs.map((x) => x.id.replace(/(^|->)[^>]*?::/g, "$1"));

// module.vpc の中に aws_vpc.this と module.vpc.module.sub（その中に aws_subnet.this）
const vpc = n("module.vpc");
const sub = n("module.vpc.module.sub", { kind: "module", parentModule: vpc.id });
const modGraph = graph(
  [
    n("aws_instance.web", { type: "aws_instance" }),
    n("var.cidr"),
    n("local.name"),
    vpc,
    n("module.vpc.aws_vpc.this", { type: "aws_vpc", parentModule: vpc.id }),
    n("module.vpc.var.cidr", { kind: "variable", parentModule: vpc.id }),
    sub,
    n("module.vpc.module.sub.aws_subnet.this", { type: "aws_subnet", parentModule: sub.id }),
    n("module.ext", { external: true }),
  ],
  [
    e("aws_instance.web", "module.vpc", { kind: "module_io", port: "subnet_ids", attr: "subnet_id" }),
    e("aws_instance.web", "local.name", { attr: "tags" }),
    e("local.name", "var.cidr"),
    e("module.vpc", "var.cidr", { kind: "module_io", attr: "cidr" }),
    e("module.vpc.aws_vpc.this", "module.vpc.var.cidr"),
    e("module.vpc.module.sub", "module.vpc.aws_vpc.this", { kind: "module_io", attr: "vpc_id" }),
    e("module.vpc.module.sub.aws_subnet.this", "module.vpc.aws_vpc.this"),
    e("module.ext", "module.vpc", { kind: "module_io", port: "vpc_id" }),
  ],
);

describe("buildViewGraph", () => {
  it("初期表示: モジュールは折りたたみ、var / local は非表示", () => {
    const v = buildViewGraph(modGraph, opts());
    expect(ids(v.nodes)).toEqual(["aws_instance.web", "module.ext", "module.vpc"]);
    expect(v.nodes.every((x) => !x.isGroup)).toBe(true);
    // local.name → var.cidr はどちらも非表示で、その先に表示ノードがないのでエッジは消える
    expect(ids(v.edges)).toEqual(["aws_instance.web->module.vpc", "module.ext->module.vpc"]);
  });

  it("モジュールのリソース数は入れ子を含む", () => {
    const v = buildViewGraph(modGraph, opts());
    expect(v.nodes.find((x) => x.id === vpc.id)?.resourceCount).toBe(2);
    expect(v.nodes.find((x) => x.id === vpc.id)?.expandable).toBe(true);
    expect(v.nodes.find((x) => x.id === `${R}::module.ext`)?.expandable).toBe(false);
  });

  it("var / local を経由した依存を直接のエッジにまとめる", () => {
    const g = graph(
      [n("aws_a.x"), n("local.l1"), n("local.l2"), n("var.v"), n("aws_b.y"), n("aws_c.z")],
      [e("aws_a.x", "local.l1", { attr: "name" }), e("local.l1", "local.l2"), e("local.l2", "aws_b.y"), e("local.l2", "var.v"), e("var.v", "aws_c.z")],
    );
    const v = buildViewGraph(g, opts());
    expect(ids(v.edges)).toEqual(["aws_a.x->aws_b.y", "aws_a.x->aws_c.z"]);
    expect(v.edges.every((x) => x.indirect)).toBe(true);
    // 元のエッジ（参照元側）の属性が残る
    expect(v.edges[0].edges[0].attr).toBe("name");

    const shown = buildViewGraph(g, opts({ showLocals: true }));
    expect(ids(shown.edges)).toEqual(["aws_a.x->local.l1", "local.l1->local.l2", "local.l2->aws_b.y", "local.l2->aws_c.z"]);
  });

  it("循環していても止まる", () => {
    const g = graph([n("aws_a.x"), n("local.a"), n("local.b")], [e("aws_a.x", "local.a"), e("local.a", "local.b"), e("local.b", "local.a")]);
    expect(buildViewGraph(g, opts()).edges).toEqual([]);
  });

  it("モジュールを展開すると子がグループに入る", () => {
    const v = buildViewGraph(modGraph, opts({ expanded: new Set([vpc.id]) }));
    expect(ids(v.nodes)).toEqual(["aws_instance.web", "module.ext", "module.vpc", "module.vpc.aws_vpc.this", "module.vpc.module.sub"]);
    const group = v.nodes.find((x) => x.id === vpc.id)!;
    expect(group.isGroup).toBe(true);
    // 親が子より先に並ぶ
    const idx = (id: string) => v.nodes.findIndex((x) => x.id === id);
    expect(idx(vpc.id)).toBeLessThan(idx(`${R}::module.vpc.aws_vpc.this`));
    expect(v.nodes.find((x) => x.id === `${R}::module.vpc.aws_vpc.this`)?.parent).toBe(vpc.id);
    // 折りたたんだ入れ子モジュールの中のノードは、そのモジュールにまとめる
    expect(ids(v.edges)).toContain("module.vpc.module.sub->module.vpc.aws_vpc.this");
    expect(v.representative.get(`${R}::module.vpc.module.sub.aws_subnet.this`)).toBe(sub.id);
  });

  it("入れ子の展開", () => {
    const v = buildViewGraph(modGraph, opts({ expanded: new Set([vpc.id, sub.id]) }));
    expect(ids(v.nodes)).toContain("module.vpc.module.sub.aws_subnet.this");
    expect(ids(v.edges)).toContain("module.vpc.module.sub.aws_subnet.this->module.vpc.aws_vpc.this");
    // 外側が折りたたまれていれば内側の展開状態は無視する
    const outerCollapsed = buildViewGraph(modGraph, opts({ expanded: new Set([sub.id]) }));
    expect(ids(outerCollapsed.nodes)).not.toContain("module.vpc.module.sub");
  });

  it("far 用のエッジはトップレベルのグループ間で1本に集約する", () => {
    const v = buildViewGraph(modGraph, opts({ expanded: new Set([vpc.id, sub.id]) }));
    expect(ids(v.groupEdges)).toEqual(["aws_instance.web->module.vpc", "module.ext->module.vpc"]);
  });

  it("リソース種別フィルタ", () => {
    const g = graph(
      [n("aws_a.x", { type: "aws_a" }), n("aws_b.y", { type: "aws_b" }), n("aws_c.z", { type: "aws_c" })],
      [e("aws_a.x", "aws_b.y"), e("aws_b.y", "aws_c.z")],
    );
    const v = buildViewGraph(g, opts({ hiddenTypes: new Set(["aws_b"]) }));
    expect(ids(v.nodes)).toEqual(["aws_a.x", "aws_c.z"]);
    expect(v.edges).toEqual([]);
    expect(v.representative.get(`${R}::aws_b.y`)).toBeNull();
  });

  it("同じ表示エッジにまとまった元のエッジと種別を保持する", () => {
    const g = graph(
      [n("aws_a.x"), n("aws_b.y")],
      [e("aws_a.x", "aws_b.y", { attr: "a" }), e("aws_a.x", "aws_b.y", { attr: "b", kind: "depends_on" })],
    );
    const v = buildViewGraph(g, opts());
    expect(v.edges).toHaveLength(1);
    expect(v.edges[0].edges.map((x) => x.attr)).toEqual(["a", "b"]);
    expect(v.edges[0].kinds).toEqual(["depends_on", "reference"]);
  });
});

describe("helpers", () => {
  it("ancestorModules", () => {
    expect(ancestorModules(modGraph, `${R}::module.vpc.module.sub.aws_subnet.this`)).toEqual([vpc.id, sub.id]);
  });
  it("resourceTypes", () => {
    expect(resourceTypes(modGraph)).toEqual(["aws_instance", "aws_subnet", "aws_vpc"]);
  });
});

describe("Go のゴールデン出力に対して", () => {
  const file = fileURLToPath(new URL("../../../testdata/golden/repo/basic/envs__dev__network.json", import.meta.url));
  const g = JSON.parse(readFileSync(file, "utf8")) as RootGraph;

  it("すべて展開・表示すると元のノードがすべて出る", () => {
    const all = new Set(g.nodes.filter((x) => x.kind === "module").map((x) => x.id));
    const v = buildViewGraph(g, opts({ expanded: all, showLocals: true, showVariables: true }));
    expect(v.nodes).toHaveLength(g.nodes.length);
    expect(v.edges).toHaveLength(new Set(g.edges.map((x) => `${x.from}->${x.to}`)).size);
  });

  it("初期表示ではトップレベルの resource / data / module / output だけ", () => {
    const v = buildViewGraph(g, opts());
    for (const x of v.nodes) {
      expect(x.node.parentModule).toBeUndefined();
      expect(["resource", "data", "module", "output"]).toContain(x.node.kind);
    }
    // module.vpc → local.azs → data.aws_availability_zones.available が直接のエッジになる
    expect(ids(v.edges)).toContain("module.vpc->data.aws_availability_zones.available");
  });
});
