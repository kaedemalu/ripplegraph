"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useSearch } from "@/lib/queries";
import { rootHref } from "@/lib/urls";

/** 全ルートのリソースアドレスを検索する。一致したルートを onHits で親に知らせる（セルの強調用）。 */
export function SearchBox({ currentRef, onHits }: { currentRef: string; onHits: (roots: Map<string, number>) => void }) {
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setQ(input.trim()), 250);
    return () => clearTimeout(t);
  }, [input]);

  const search = useSearch(q, currentRef);
  const hits = q.length >= 2 ? (search.data?.hits ?? []) : [];

  useEffect(() => {
    const m = new Map<string, number>();
    for (const h of hits) m.set(h.root, (m.get(h.root) ?? 0) + 1);
    onHits(m);
    // hits は search.data から導出しているので、その参照の変化だけを見る
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search.data, q, onHits]);

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  return (
    <div className="search" ref={boxRef}>
      <input
        className="input"
        type="search"
        placeholder="リソースアドレスを検索（例: aws_subnet）"
        value={input}
        onChange={(e) => {
          setInput(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
        style={{ width: 300 }}
      />
      {open && q.length >= 2 && (
        <div className="search-results">
          {search.isFetching && hits.length === 0 && <div className="search-empty">検索中…</div>}
          {!search.isFetching && hits.length === 0 && <div className="search-empty">一致するリソースはありません</div>}
          {hits.map((h) => (
            <Link key={h.nodeId} href={rootHref(h.root, currentRef, h.address)} className="search-hit" onClick={() => setOpen(false)}>
              <span className="mono">{h.address}</span>
              <span className="search-root">{h.root}</span>
            </Link>
          ))}
          {search.data?.truncated && <div className="search-empty">件数が多いため一部のみ表示しています</div>}
        </div>
      )}
    </div>
  );
}
