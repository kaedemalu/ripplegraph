"use client";

import { useEffect, useRef, useState } from "react";

/** リソース種別フィルタ（チェックを外した種別を非表示にする）。 */
export function TypeFilter({ types, hidden, onChange }: { types: string[]; hidden: ReadonlySet<string>; onChange: (hidden: Set<string>) => void }) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  const shown = types.filter((t) => t.includes(q.trim()));
  const toggle = (t: string) => {
    const next = new Set(hidden);
    if (next.has(t)) next.delete(t);
    else next.add(t);
    onChange(next);
  };

  return (
    <div className="popover-anchor" ref={ref}>
      <button className="btn" aria-pressed={hidden.size > 0} onClick={() => setOpen((o) => !o)} disabled={types.length === 0}>
        種別{hidden.size > 0 ? `（${types.length - hidden.size}/${types.length}）` : ""} ▾
      </button>
      {open && (
        <div className="popover">
          <input className="input" placeholder="種別を絞り込み" value={q} onChange={(e) => setQ(e.target.value)} autoFocus />
          <div className="popover-actions">
            <button className="linklike" onClick={() => onChange(new Set())}>
              すべて表示
            </button>
            <button className="linklike" onClick={() => onChange(new Set(types))}>
              すべて非表示
            </button>
          </div>
          <ul className="type-list">
            {shown.map((t) => (
              <li key={t}>
                <label>
                  <input type="checkbox" checked={!hidden.has(t)} onChange={() => toggle(t)} />
                  <span className="mono">{t}</span>
                </label>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
