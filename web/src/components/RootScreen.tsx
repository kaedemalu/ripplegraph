"use client";

import { ReactFlowProvider } from "@xyflow/react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DetailPanel } from "@/components/DetailPanel";
import { Diagnostics } from "@/components/Diagnostics";
import { EdgeInfo } from "@/components/EdgeInfo";
import { GraphCanvas, type FocusRequest } from "@/components/graph/GraphCanvas";
import { NodeSearch } from "@/components/NodeSearch";
import { RefSelect } from "@/components/RefSelect";
import { TopBar } from "@/components/TopBar";
import { TypeFilter } from "@/components/TypeFilter";
import { useLayout } from "@/lib/layoutClient";
import { useRefs, useRootGraph } from "@/lib/queries";
import { addressOf, matrixHref } from "@/lib/urls";
import { useUrlState } from "@/lib/useUrlState";
import {
  ancestorModules,
  buildViewGraph,
  defaultViewOptions,
  resourceTypes,
  viewOptionsKey,
  type ViewEdge,
  type ViewOptions,
} from "@/lib/viewGraph";
import type { RootGraph } from "@/types/graph";

export function RootScreen() {
  const url = useUrlState();
  const path = url.get("path");
  const ref = url.get("ref");
  const refs = useRefs();
  const graph = useRootGraph(path, ref);

  return (
    <div className="app">
      <TopBar currentRef={ref} showRepo={false}>
        <nav className="crumbs">
          <span className="sep">/</span>
          <Link href={matrixHref(ref)}>{refs.data?.repo ?? "リポジトリ"}</Link>
          <span className="sep">/</span>
          <span className="current mono">{path || "（path 未指定）"}</span>
        </nav>
        <div className="spacer" />
        <RefSelect value={ref} onChange={(r) => url.set({ ref: r })} />
      </TopBar>
      {!path && <div className="error-box">path パラメータを指定してください。</div>}
      {path && graph.isPending && <div className="center-msg">読み込み中…</div>}
      {graph.isError && <div className="error-box">読み込めませんでした: {graph.error.message}</div>}
      {graph.data && (
        <ReactFlowProvider>
          {/* ルート・SHA が変わったら表示状態（展開状態など）をリセットする */}
          <RootGraphView key={`${graph.data.sha}|${graph.data.root}`} graph={graph.data} />
        </ReactFlowProvider>
      )}
    </div>
  );
}

function RootGraphView({ graph }: { graph: RootGraph }) {
  const url = useUrlState();
  const nodeParam = url.get("node");
  const [opts, setOpts] = useState<ViewOptions>(defaultViewOptions);
  const [focus, setFocus] = useState<FocusRequest>(null);
  const [hoverEdge, setHoverEdge] = useState<ViewEdge | null>(null);
  const [pinnedEdge, setPinnedEdge] = useState<ViewEdge | null>(null);
  const seq = useRef(0);

  const byId = useMemo(() => new Map(graph.nodes.map((n) => [n.id, n])), [graph]);
  const types = useMemo(() => resourceTypes(graph), [graph]);
  const modules = useMemo(() => graph.nodes.filter((n) => n.kind === "module" && !n.external).map((n) => n.id), [graph]);

  const keyOf = useCallback((o: ViewOptions) => `${graph.sha}|${graph.root}|${viewOptionsKey(o)}`, [graph]);
  const targetView = useMemo(() => buildViewGraph(graph, opts), [graph, opts]);
  const targetKey = keyOf(opts);
  const { view, layout, layoutKey, loading, error } = useLayout(targetKey, targetView);

  const selectedId = nodeParam ? `${graph.root}::${nodeParam}` : null;
  const selectedNode = selectedId ? byId.get(selectedId) : undefined;
  const selectedVisible = selectedId && view ? (view.representative.get(selectedId) ?? null) : null;

  /** ノードを表示できるように展開・表示切り替えをして、選択してズームする */
  const focusNode = useCallback(
    (id: string) => {
      const n = byId.get(id);
      if (!n) return;
      let next = opts;
      const ancestors = ancestorModules(graph, id);
      if (ancestors.some((a) => !opts.expanded.has(a))) {
        next = { ...next, expanded: new Set([...opts.expanded, ...ancestors]) };
      }
      if (n.kind === "variable" && !next.showVariables) next = { ...next, showVariables: true };
      if (n.kind === "local" && !next.showLocals) next = { ...next, showLocals: true };
      if (n.type && next.hiddenTypes.has(n.type)) {
        const hiddenTypes = new Set(next.hiddenTypes);
        hiddenTypes.delete(n.type);
        next = { ...next, hiddenTypes };
      }
      if (next !== opts) setOpts(next);
      setFocus({ id, seq: ++seq.current, layoutKey: keyOf(next) });
      url.set({ node: addressOf(id) }, { replace: true });
    },
    [byId, graph, opts, keyOf, url],
  );

  // URL で指定されたノード（検索結果から来た場合など）へ最初に一度だけズームする
  const initialFocused = useRef(false);
  useEffect(() => {
    if (initialFocused.current) return;
    initialFocused.current = true;
    if (selectedId && byId.has(selectedId)) focusNode(selectedId);
  }, [selectedId, byId, focusNode]);

  const onSelect = useCallback(
    (id: string | null) => {
      if (!id) {
        url.set({ node: undefined }, { replace: true });
        return;
      }
      url.set({ node: addressOf(id) }, { replace: true });
    },
    [url],
  );

  const toggleModule = useCallback((id: string) => {
    setOpts((o) => {
      const expanded = new Set(o.expanded);
      if (expanded.has(id)) expanded.delete(id);
      else expanded.add(id);
      return { ...o, expanded };
    });
  }, []);

  const shownEdge = hoverEdge ?? pinnedEdge;

  return (
    <>
      <div className="toolbar">
        <button className="btn" aria-pressed={opts.showVariables} onClick={() => setOpts((o) => ({ ...o, showVariables: !o.showVariables }))}>
          var
        </button>
        <button className="btn" aria-pressed={opts.showLocals} onClick={() => setOpts((o) => ({ ...o, showLocals: !o.showLocals }))}>
          local
        </button>
        <span className="toolbar-sep" />
        <button className="btn" disabled={modules.length === 0} onClick={() => setOpts((o) => ({ ...o, expanded: new Set(modules) }))}>
          すべて展開
        </button>
        <button className="btn" disabled={opts.expanded.size === 0} onClick={() => setOpts((o) => ({ ...o, expanded: new Set() }))}>
          すべて折りたたむ
        </button>
        <span className="toolbar-sep" />
        <TypeFilter types={types} hidden={opts.hiddenTypes} onChange={(hiddenTypes) => setOpts((o) => ({ ...o, hiddenTypes }))} />
        <span className="toolbar-sep" />
        <NodeSearch graph={graph} onPick={focusNode} />
        <div className="spacer" />
        <span className="toolbar-stats">
          {view ? `${view.nodes.length} ノード / ${view.edges.length} エッジ` : ""}
          {layout ? ` · レイアウト ${Math.round(layout.elapsedMs)} ms` : ""}
        </span>
      </div>
      <div className="graph-layout">
        <div className="graph-canvas">
          {view && layout && layoutKey ? (
            <GraphCanvas
              view={view}
              layout={layout}
              layoutKey={layoutKey}
              selectedId={selectedVisible}
              focus={focus}
              fitKey={`${graph.sha}|${graph.root}`}
              onSelect={onSelect}
              onToggleModule={toggleModule}
              onEdgeHover={setHoverEdge}
              onEdgePin={setPinnedEdge}
            />
          ) : (
            !error && <div className="center-msg">レイアウトを計算しています…（{targetView.nodes.length} ノード）</div>
          )}
          {loading && view && <div className="layout-busy">レイアウトを計算しています…</div>}
          {error && <div className="error-box">レイアウトの計算に失敗しました: {error}</div>}
          {shownEdge && <EdgeInfo edge={shownEdge} nodes={byId} pinned={!hoverEdge && !!pinnedEdge} onClose={() => setPinnedEdge(null)} />}
          <Legend />
        </div>
        <aside className="graph-side">
          {selectedNode ? (
            <DetailPanel graph={graph} node={selectedNode} onPick={focusNode} onClose={() => onSelect(null)} />
          ) : (
            <RootSummary graph={graph} />
          )}
        </aside>
      </div>
    </>
  );
}

function RootSummary({ graph }: { graph: RootGraph }) {
  const count = (k: string) => graph.nodes.filter((n) => n.kind === k).length;
  return (
    <div className="detail">
      <h2 className="detail-title mono">{graph.root}</h2>
      {(graph.env || graph.component) && (
        <div className="detail-sub">
          {graph.env && <span className="badge">env: {graph.env}</span>} {graph.component && <span className="badge">component: {graph.component}</span>}
        </div>
      )}
      <dl className="detail-grid">
        <dt>resource</dt>
        <dd>{count("resource")}</dd>
        <dt>data</dt>
        <dd>{count("data")}</dd>
        <dt>module</dt>
        <dd>{count("module")}</dd>
        <dt>variable / local / output</dt>
        <dd>
          {count("variable")} / {count("local")} / {count("output")}
        </dd>
        <dt>SHA</dt>
        <dd className="mono">{graph.sha === "worktree" ? "ワーキングツリー" : graph.sha.slice(0, 12)}</dd>
      </dl>
      <p className="hint">
        ノードをクリックすると詳細を表示します。モジュールはダブルクリックか ▸ で展開できます。エッジにカーソルを合わせると参照している属性を表示します。
      </p>
      <Diagnostics diagnostics={graph.diagnostics} open={graph.diagnostics.some((d) => d.severity === "error")} />
    </div>
  );
}

function Legend() {
  return (
    <div className="legend">
      <span>
        <i className="lg-edge kind-reference" /> 参照
      </span>
      <span>
        <i className="lg-edge kind-module_io" /> モジュール入出力
      </span>
      <span>
        <i className="lg-edge kind-depends_on" /> depends_on
      </span>
      <span>
        <i className="lg-edge kind-replace_triggered_by" /> replace_triggered_by
      </span>
      <span>
        <i className="lg-edge indirect" /> var / local 経由
      </span>
      <span className="legend-dir">矢印: 依存する側 → 依存される側</span>
    </div>
  );
}
