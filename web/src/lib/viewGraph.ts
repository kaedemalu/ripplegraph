/**
 * 取得した RootGraph に、モジュールの折りたたみ状態・var / local の表示切り替え・
 * リソース種別フィルタを適用して「表示するノードとエッジ」を作る純粋関数。
 */
import type { Edge, EdgeKind, Node, RootGraph } from "@/types/graph";

export type ViewOptions = {
  /** var.* を表示する（非表示なら、それを経由した依存を直接のエッジにまとめる） */
  showVariables: boolean;
  /** local.* を表示する（同上） */
  showLocals: boolean;
  /** 展開しているモジュールノードの id */
  expanded: ReadonlySet<string>;
  /** 非表示にするリソース種別（resource / data の type） */
  hiddenTypes: ReadonlySet<string>;
};

export const defaultViewOptions: ViewOptions = {
  showVariables: false,
  showLocals: false,
  expanded: new Set(),
  hiddenTypes: new Set(),
};

export type ViewNode = {
  id: string;
  node: Node;
  /** 親グループ（展開しているモジュール）の id */
  parent?: string;
  /** 展開していて、表示する子を持つモジュール */
  isGroup: boolean;
  /** 展開できるモジュール（中身が見えるもの） */
  expandable: boolean;
  /** モジュール配下（入れ子を含む）の resource + data の数 */
  resourceCount: number;
  /** 入れ子の深さ（0 がルート直下） */
  depth: number;
};

export type ViewEdge = {
  id: string;
  source: string;
  target: string;
  /** 元のエッジ（参照元側の最初の1本）。属性・ファイル・行の表示に使う */
  edges: Edge[];
  kinds: EdgeKind[];
  /** 非表示の var / local を経由してまとめたエッジを含む */
  indirect: boolean;
};

export type ViewGraph = {
  nodes: ViewNode[];
  edges: ViewEdge[];
  /** 縮小表示（far）用に、トップレベルのグループ間で1本に集約したエッジ */
  groupEdges: ViewEdge[];
  /** 元のノード id → 表示上の代表ノード id（非表示なら null） */
  representative: Map<string, string | null>;
};

type Rep = { kind: "node"; id: string } | { kind: "bypass" } | { kind: "drop" };

const KIND_ORDER: EdgeKind[] = ["depends_on", "replace_triggered_by", "module_io", "reference"];

export function buildViewGraph(graph: RootGraph, opts: ViewOptions): ViewGraph {
  const byId = new Map<string, Node>();
  for (const n of graph.nodes) byId.set(n.id, n);

  const children = new Map<string, Node[]>();
  for (const n of graph.nodes) {
    if (!n.parentModule) continue;
    const list = children.get(n.parentModule) ?? [];
    list.push(n);
    children.set(n.parentModule, list);
  }

  // 祖先の module id（外側から順）
  const ancestorsCache = new Map<string, string[]>();
  const ancestors = (n: Node): string[] => {
    const cached = ancestorsCache.get(n.id);
    if (cached) return cached;
    const parent = n.parentModule ? byId.get(n.parentModule) : undefined;
    const list = parent ? [...ancestors(parent), parent.id] : [];
    ancestorsCache.set(n.id, list);
    return list;
  };

  const countCache = new Map<string, number>();
  const resourceCount = (id: string): number => {
    const cached = countCache.get(id);
    if (cached !== undefined) return cached;
    let c = 0;
    for (const ch of children.get(id) ?? []) {
      if (ch.kind === "resource" || ch.kind === "data") c++;
      if (ch.kind === "module") c += resourceCount(ch.id);
    }
    countCache.set(id, c);
    return c;
  };

  const repCache = new Map<string, Rep>();
  const rep = (id: string): Rep => {
    const cached = repCache.get(id);
    if (cached) return cached;
    const n = byId.get(id);
    let r: Rep;
    if (!n) {
      r = { kind: "drop" };
    } else {
      const collapsed = ancestors(n).find((a) => !opts.expanded.has(a));
      if (collapsed) r = { kind: "node", id: collapsed };
      else if (n.kind === "variable" && !opts.showVariables) r = { kind: "bypass" };
      else if (n.kind === "local" && !opts.showLocals) r = { kind: "bypass" };
      else if ((n.kind === "resource" || n.kind === "data") && n.type && opts.hiddenTypes.has(n.type)) r = { kind: "drop" };
      else r = { kind: "node", id };
    }
    repCache.set(id, r);
    return r;
  };

  const outEdges = new Map<string, Edge[]>();
  for (const e of graph.edges) {
    const list = outEdges.get(e.from) ?? [];
    list.push(e);
    outEdges.set(e.from, list);
  }

  // 非表示の var / local から到達できる表示ノード（メモ化。循環は途中で打ち切る）
  const reachCache = new Map<string, Set<string>>();
  const reachThrough = (id: string, visiting: Set<string>): Set<string> => {
    const cached = reachCache.get(id);
    if (cached) return cached;
    if (visiting.has(id)) return new Set();
    visiting.add(id);
    const out = new Set<string>();
    for (const e of outEdges.get(id) ?? []) {
      const r = rep(e.to);
      if (r.kind === "node") out.add(r.id);
      else if (r.kind === "bypass") for (const x of reachThrough(e.to, visiting)) out.add(x);
    }
    visiting.delete(id);
    reachCache.set(id, out);
    return out;
  };

  // 表示ノード
  const visible = new Set<string>();
  for (const n of graph.nodes) {
    const r = rep(n.id);
    if (r.kind === "node" && r.id === n.id) visible.add(n.id);
  }

  // エッジ
  const edgeMap = new Map<string, ViewEdge>();
  const addEdge = (source: string, target: string, e: Edge, indirect: boolean) => {
    if (source === target) return;
    const id = `${source}->${target}`;
    let ve = edgeMap.get(id);
    if (!ve) {
      ve = { id, source, target, edges: [], kinds: [], indirect: false };
      edgeMap.set(id, ve);
    }
    if (!ve.edges.includes(e)) ve.edges.push(e);
    if (!ve.kinds.includes(e.kind)) ve.kinds.push(e.kind);
    ve.indirect ||= indirect;
  };
  for (const e of graph.edges) {
    const rs = rep(e.from);
    if (rs.kind !== "node") continue;
    const rt = rep(e.to);
    if (rt.kind === "node") addEdge(rs.id, rt.id, e, false);
    else if (rt.kind === "bypass") for (const t of reachThrough(e.to, new Set())) addEdge(rs.id, t, e, true);
  }
  const edges = [...edgeMap.values()].sort((a, b) => cmp(a.id, b.id));
  for (const ve of edges) ve.kinds.sort((a, b) => KIND_ORDER.indexOf(a) - KIND_ORDER.indexOf(b));

  // グループ（展開していて、表示する子を持つモジュール）
  const hasVisibleChild = new Set<string>();
  for (const id of visible) {
    const p = byId.get(id)?.parentModule;
    if (p) hasVisibleChild.add(p);
  }

  const nodes: ViewNode[] = [];
  for (const id of visible) {
    const n = byId.get(id)!;
    const isModule = n.kind === "module";
    const expandable = isModule && !n.external && (children.get(id)?.length ?? 0) > 0;
    nodes.push({
      id,
      node: n,
      parent: n.parentModule,
      isGroup: expandable && opts.expanded.has(id) && hasVisibleChild.has(id),
      expandable,
      resourceCount: isModule ? resourceCount(id) : 0,
      depth: ancestors(n).length,
    });
  }
  // 親が子より先に来るようにする（React Flow の要件）
  nodes.sort((a, b) => a.depth - b.depth || cmp(a.id, b.id));

  // far 用: トップレベルのグループ間に集約したエッジ
  const top = (id: string): string => {
    const n = byId.get(id);
    if (!n) return id;
    return ancestors(n)[0] ?? id;
  };
  const groupMap = new Map<string, ViewEdge>();
  for (const ve of edges) {
    const s = top(ve.source);
    const t = top(ve.target);
    if (s === t) continue;
    const id = `${s}->${t}`;
    let g = groupMap.get(id);
    if (!g) {
      g = { id, source: s, target: t, edges: [], kinds: [], indirect: false };
      groupMap.set(id, g);
    }
    for (const e of ve.edges) if (!g.edges.includes(e)) g.edges.push(e);
    for (const k of ve.kinds) if (!g.kinds.includes(k)) g.kinds.push(k);
    g.indirect ||= ve.indirect;
  }
  const groupEdges = [...groupMap.values()].sort((a, b) => cmp(a.id, b.id));
  for (const ve of groupEdges) ve.kinds.sort((a, b) => KIND_ORDER.indexOf(a) - KIND_ORDER.indexOf(b));

  const representative = new Map<string, string | null>();
  for (const n of graph.nodes) {
    const r = rep(n.id);
    representative.set(n.id, r.kind === "node" ? r.id : null);
  }

  return { nodes, edges, groupEdges, representative };
}

function cmp(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** ノードを表示するために展開が必要な祖先モジュールの id（外側から順）。 */
export function ancestorModules(graph: RootGraph, id: string): string[] {
  const byId = new Map(graph.nodes.map((n) => [n.id, n]));
  const out: string[] = [];
  let cur = byId.get(id)?.parentModule;
  while (cur) {
    out.unshift(cur);
    cur = byId.get(cur)?.parentModule;
  }
  return out;
}

/** グラフに含まれる resource / data の種別（昇順）。 */
export function resourceTypes(graph: RootGraph): string[] {
  const s = new Set<string>();
  for (const n of graph.nodes) if ((n.kind === "resource" || n.kind === "data") && n.type) s.add(n.type);
  return [...s].sort();
}

/** レイアウトのキャッシュキーに使う表示オプションの文字列表現。 */
export function viewOptionsKey(opts: ViewOptions): string {
  return JSON.stringify([opts.showVariables, opts.showLocals, [...opts.expanded].sort(), [...opts.hiddenTypes].sort()]);
}
