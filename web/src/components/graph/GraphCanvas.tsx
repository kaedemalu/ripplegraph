"use client";

import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  ReactFlow,
  useReactFlow,
  type Edge as FlowEdge,
  type Node as FlowNode,
} from "@xyflow/react";
import { useCallback, useEffect, useMemo, useRef } from "react";
import type { Layout } from "@/lib/layoutClient";
import { NODE_HEIGHT, NODE_WIDTH } from "@/lib/layoutTypes";
import type { ViewEdge, ViewGraph } from "@/lib/viewGraph";
import type { EdgeKind } from "@/types/graph";
import { GraphActionsContext, type GraphActions } from "./actions";
import { nodeTypeOf, nodeTypes, type GraphFlowNode } from "./nodes";
import { useZoomStage } from "./zoom";

// マーカーは SVG の属性に色を直接書くため、CSS 変数ではなく両テーマで見える中間色を使う
const EDGE_COLORS: Record<EdgeKind, string> = {
  reference: "#8b94a5",
  module_io: "#8f6ad8",
  depends_on: "#d08a1c",
  replace_triggered_by: "#d9534b",
};

/** 指定ノードへのズーム要求。layoutKey のレイアウトが出た時点で実行する。 */
export type FocusRequest = { id: string; seq: number; layoutKey: string } | null;

type Props = {
  view: ViewGraph;
  layout: Layout;
  layoutKey: string;
  /** 選択中の表示ノード（代表ノード）の id */
  selectedId: string | null;
  focus: FocusRequest;
  /** 変わったら全体表示し直す（ルートや ref が変わったとき） */
  fitKey: string;
  onSelect: (id: string | null) => void;
  onToggleModule: (id: string) => void;
  onEdgeHover: (edge: ViewEdge | null) => void;
  onEdgePin: (edge: ViewEdge | null) => void;
};

export function GraphCanvas({ view, layout, layoutKey, selectedId, focus, fitKey, onSelect, onToggleModule, onEdgeHover, onEdgePin }: Props) {
  const rf = useReactFlow();
  const stage = useZoomStage();

  const baseNodes = useMemo<GraphFlowNode[]>(
    () =>
      view.nodes.map((v) => {
        const p = layout.positions[v.id] ?? { x: 0, y: 0, width: NODE_WIDTH, height: NODE_HEIGHT };
        return {
          id: v.id,
          type: nodeTypeOf(v),
          position: { x: p.x, y: p.y },
          parentId: v.parent,
          data: { view: v },
          width: p.width,
          height: p.height,
          draggable: false,
          connectable: false,
        };
      }),
    [view, layout],
  );

  const viewEdges = stage === "far" ? view.groupEdges : view.edges;

  // 選択ノードと、それにつながるノード・エッジ
  const related = useMemo(() => {
    if (!selectedId) return null;
    const nodes = new Set<string>([selectedId]);
    const edges = new Set<string>();
    for (const e of viewEdges) {
      if (e.source === selectedId || e.target === selectedId) {
        nodes.add(e.source);
        nodes.add(e.target);
        edges.add(e.id);
      }
    }
    return { nodes, edges };
  }, [selectedId, viewEdges]);

  const nodes = useMemo<FlowNode[]>(() => {
    if (!related) return baseNodes;
    return baseNodes.map((n) => {
      const cls = n.id === selectedId ? "is-selected" : related.nodes.has(n.id) ? "is-related" : "is-dimmed";
      return { ...n, className: cls, selected: n.id === selectedId };
    });
  }, [baseNodes, related, selectedId]);

  const edges = useMemo<FlowEdge[]>(
    () =>
      viewEdges.map((e) => {
        const kind = e.kinds[0] ?? "reference";
        const state = related ? (related.edges.has(e.id) ? " is-related" : " is-dimmed") : "";
        return {
          id: e.id,
          source: e.source,
          target: e.target,
          className: `kind-${kind}${e.indirect ? " indirect" : ""}${state}`,
          markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color: EDGE_COLORS[kind] },
          data: { view: e },
          focusable: false,
        };
      }),
    [viewEdges, related],
  );

  const edgeById = useMemo(() => new Map(viewEdges.map((e) => [e.id, e])), [viewEdges]);

  // 全体表示（ルート・ref が変わって最初のレイアウトが出たとき）
  const fittedKey = useRef<string | null>(null);
  const focusedSeq = useRef<number>(-1);
  const focusPending = focus !== null && focusedSeq.current !== focus.seq;
  useEffect(() => {
    if (fittedKey.current === fitKey || focusPending) return;
    fittedKey.current = fitKey;
    requestAnimationFrame(() => rf.fitView({ padding: 0.08, maxZoom: 1.2 }));
  }, [fitKey, layout, rf, focusPending]);

  // 指定ノードへズーム（要求したレイアウトが出た時点で実行する）
  useEffect(() => {
    if (!focus || focusedSeq.current === focus.seq || focus.layoutKey !== layoutKey) return;
    focusedSeq.current = focus.seq;
    fittedKey.current = fitKey;
    const target = view.representative.get(focus.id);
    const p = target ? layout.positions[target] : undefined;
    if (!target || !p) {
      // 表示されないノード（フィルタで非表示など）は全体表示にする
      requestAnimationFrame(() => rf.fitView({ padding: 0.08, maxZoom: 1.2 }));
      return;
    }
    // 親グループの座標を足して絶対座標にする
    let x = p.x + p.width / 2;
    let y = p.y + p.height / 2;
    let parent = view.nodes.find((n) => n.id === target)?.parent;
    while (parent) {
      const pp = layout.positions[parent];
      if (!pp) break;
      x += pp.x;
      y += pp.y;
      parent = view.nodes.find((n) => n.id === parent)?.parent;
    }
    requestAnimationFrame(() => rf.setCenter(x, y, { zoom: Math.max(rf.getZoom(), 1), duration: 400 }));
  }, [focus, layout, layoutKey, view, rf, fitKey]);

  const actions = useMemo<GraphActions>(() => ({ toggleModule: onToggleModule }), [onToggleModule]);

  const onNodeClick = useCallback((_: React.MouseEvent, n: FlowNode) => onSelect(n.id), [onSelect]);
  const onNodeDoubleClick = useCallback(
    (_: React.MouseEvent, n: FlowNode) => {
      const v = (n as GraphFlowNode).data.view;
      if (v.expandable) onToggleModule(n.id);
    },
    [onToggleModule],
  );
  const onPaneClick = useCallback(() => {
    onSelect(null);
    onEdgePin(null);
  }, [onSelect, onEdgePin]);

  return (
    <GraphActionsContext.Provider value={actions}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onlyRenderVisibleElements
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        minZoom={0.05}
        maxZoom={2.5}
        onNodeClick={onNodeClick}
        onNodeDoubleClick={onNodeDoubleClick}
        onPaneClick={onPaneClick}
        onEdgeMouseEnter={(_, e) => onEdgeHover(edgeById.get(e.id) ?? null)}
        onEdgeMouseLeave={() => onEdgeHover(null)}
        onEdgeClick={(_, e) => onEdgePin(edgeById.get(e.id) ?? null)}
        zoomOnDoubleClick={false}
        className={`stage-${stage}`}
      >
        <Background gap={24} size={1} />
        <Controls showInteractive={false} />
        <MiniMap pannable zoomable nodeColor={miniMapColor} nodeStrokeWidth={2} />
      </ReactFlow>
    </GraphActionsContext.Provider>
  );
}

function miniMapColor(n: FlowNode): string {
  const v = (n as GraphFlowNode).data?.view;
  if (!v) return "#999";
  if (v.isGroup) return "rgba(143,106,216,0.15)";
  switch (v.node.kind) {
    case "module":
      return "#8f6ad8";
    case "data":
      return "#1fa391";
    case "resource":
      return "#4c7fe8";
    default:
      return "#a0a7b4";
  }
}
