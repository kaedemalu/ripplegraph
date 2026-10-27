"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { RootGraph } from "@/types/graph";

/** ルート内のノード検索。選ぶとそのノードを展開・選択してズームする。 */
export function NodeSearch({ graph, onPick }: { graph: RootGraph; onPick: (id: string) => void }) {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const ref = useRef<HTMLDivElement>(null);

  const hits = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return [];
    return graph.nodes.filter((n) => n.address.toLowerCase().includes(s)).slice(0, 50);
  }, [graph, q]);

  useEffect(() => setActive(0), [q]);
  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  const pick = (id: string) => {
    onPick(id);
    setOpen(false);
  };

  return (
    <div className="popover-anchor" ref={ref}>
      <input
        className="input"
        type="search"
        placeholder="ノードを検索"
        value={q}
        style={{ width: 220 }}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") setActive((a) => Math.min(a + 1, hits.length - 1));
          else if (e.key === "ArrowUp") setActive((a) => Math.max(a - 1, 0));
          else if (e.key === "Enter" && hits[active]) pick(hits[active].id);
          else if (e.key === "Escape") setOpen(false);
        }}
      />
      {open && hits.length > 0 && (
        <ul className="popover node-hits">
          {hits.map((n, i) => (
            <li key={n.id}>
              <button className={`node-hit${i === active ? " active" : ""}`} onMouseEnter={() => setActive(i)} onClick={() => pick(n.id)}>
                <span className="mono">{n.address}</span>
                <span className="node-hit-kind">{n.kind}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
