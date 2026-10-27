"use client";

import { useMemo } from "react";
import { addressOf } from "@/lib/urls";
import type { Edge, Node, RootGraph } from "@/types/graph";

const KIND_LABEL: Record<string, string> = {
  reference: "参照",
  module_io: "モジュール入出力",
  depends_on: "depends_on",
  replace_triggered_by: "replace_triggered_by",
};

type Props = {
  graph: RootGraph;
  node: Node;
  onPick: (id: string) => void;
  onClose: () => void;
};

/** 選択ノードの詳細（アドレス、定義元、依存先と被依存の一覧）。 */
export function DetailPanel({ graph, node, onPick, onClose }: Props) {
  const { deps, dependents, parent, children } = useMemo(() => {
    const deps = graph.edges.filter((e) => e.from === node.id);
    const dependents = graph.edges.filter((e) => e.to === node.id);
    const parent = node.parentModule ? graph.nodes.find((n) => n.id === node.parentModule) : undefined;
    const children = node.kind === "module" ? graph.nodes.filter((n) => n.parentModule === node.id) : [];
    return { deps, dependents, parent, children };
  }, [graph, node]);

  return (
    <div className="detail">
      <div className="detail-head">
        <h2 className="detail-title mono">{node.address}</h2>
        <button className="btn btn-small" onClick={onClose} title="選択を解除">
          ×
        </button>
      </div>
      <div className="detail-sub">
        <span className={`badge kind-badge kind-${node.kind}`}>{node.kind}</span>
        {node.type && <span className="badge">{node.type}</span>}
        {node.meta?.count && <span className="badge">count</span>}
        {node.meta?.forEach && <span className="badge">for_each</span>}
        {node.external && <span className="badge">外部モジュール（中身は解決しません）</span>}
      </div>
      <dl className="detail-grid">
        <dt>定義元</dt>
        <dd className="mono">
          {node.file}:{node.line}
        </dd>
        {parent && (
          <>
            <dt>親モジュール</dt>
            <dd>
              <button className="linklike mono" onClick={() => onPick(parent.id)}>
                {parent.address}
              </button>
            </dd>
          </>
        )}
        {node.kind === "module" && !node.external && (
          <>
            <dt>中身</dt>
            <dd>{children.length} ノード</dd>
          </>
        )}
      </dl>

      <EdgeList title="依存先" edges={deps} side="to" onPick={onPick} />
      <EdgeList title="被依存" edges={dependents} side="from" onPick={onPick} />
    </div>
  );
}

function EdgeList({ title, edges, side, onPick }: { title: string; edges: Edge[]; side: "from" | "to"; onPick: (id: string) => void }) {
  return (
    <section className="edge-list">
      <h3 className="section-title">
        {title}（{edges.length}）
      </h3>
      {edges.length === 0 && <div className="hint">なし</div>}
      <ul>
        {edges.map((e, i) => {
          const other = side === "to" ? e.to : e.from;
          return (
            <li key={i}>
              <button className="edge-item" onClick={() => onPick(other)} title={`${e.file}:${e.line}`}>
                <span className="mono edge-target">{addressOf(other)}</span>
                <span className="edge-meta">
                  <span className={`edge-kind kind-${e.kind}`}>{KIND_LABEL[e.kind] ?? e.kind}</span>
                  {e.attr && (
                    <span>
                      attr: <code>{e.attr}</code>
                    </span>
                  )}
                  {e.port && (
                    <span>
                      port: <code>{e.port}</code>
                    </span>
                  )}
                  <span className="mono edge-loc">
                    {e.file}:{e.line}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
