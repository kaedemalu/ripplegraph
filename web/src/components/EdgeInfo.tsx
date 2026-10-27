"use client";

import { addressOf } from "@/lib/urls";
import type { ViewEdge } from "@/lib/viewGraph";
import type { Node } from "@/types/graph";

/** エッジのホバー・クリックで、参照している属性とファイル・行を表示する。 */
export function EdgeInfo({ edge, nodes, pinned, onClose }: { edge: ViewEdge; nodes: Map<string, Node>; pinned: boolean; onClose: () => void }) {
  const src = nodes.get(edge.source)?.address ?? addressOf(edge.source);
  const dst = nodes.get(edge.target)?.address ?? addressOf(edge.target);
  const shown = edge.edges.slice(0, 12);
  return (
    <div className="edge-info">
      <div className="edge-info-head">
        <span className="mono">
          {src} → {dst}
        </span>
        {pinned && (
          <button className="btn btn-small" onClick={onClose}>
            ×
          </button>
        )}
      </div>
      {edge.indirect && <div className="hint">非表示の var / local を経由した依存を含みます</div>}
      <ul>
        {shown.map((e, i) => (
          <li key={i}>
            <span className={`edge-kind kind-${e.kind}`}>{e.kind}</span>
            {addressOf(e.from) !== src && <span className="mono">{addressOf(e.from)}</span>}
            {e.attr && <code>{e.attr}</code>}
            {e.port && (
              <span>
                port <code>{e.port}</code>
              </span>
            )}
            {addressOf(e.to) !== dst && <span className="mono">→ {addressOf(e.to)}</span>}
            <span className="mono edge-loc">
              {e.file}:{e.line}
            </span>
          </li>
        ))}
      </ul>
      {edge.edges.length > shown.length && <div className="hint">ほか {edge.edges.length - shown.length} 件</div>}
    </div>
  );
}
