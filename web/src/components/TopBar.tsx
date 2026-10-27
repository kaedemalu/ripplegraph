"use client";

import Link from "next/link";
import { useRefs } from "@/lib/queries";
import { matrixHref } from "@/lib/urls";

/** 上部バー。showRepo=false はパンくずでリポジトリ名を出す画面用。 */
export function TopBar({ currentRef, showRepo = true, children }: { currentRef: string; showRepo?: boolean; children?: React.ReactNode }) {
  const refs = useRefs();
  return (
    <header className="topbar">
      <Link href={matrixHref(currentRef)} className="brand">
        Ripplegraph
      </Link>
      {showRepo && refs.data?.repo && <span className="mono" style={{ color: "var(--muted)" }}>{refs.data.repo}</span>}
      {children}
    </header>
  );
}
