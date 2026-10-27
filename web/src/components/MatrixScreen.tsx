"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Diagnostics } from "@/components/Diagnostics";
import { RefSelect } from "@/components/RefSelect";
import { SearchBox } from "@/components/SearchBox";
import { TopBar } from "@/components/TopBar";
import { useRepoGraph } from "@/lib/queries";
import { rootHref } from "@/lib/urls";
import { useUrlState } from "@/lib/useUrlState";
import type { RepoGraph, RepoRoot } from "@/types/graph";

// よく使われる環境名はこの順に並べ、それ以外は名前順にする
const ENV_ORDER = ["dev", "develop", "development", "test", "qa", "stg", "stage", "staging", "prd", "prod", "production"];

function sortEnvs(envs: string[]): string[] {
  return [...envs].sort((a, b) => {
    const ia = ENV_ORDER.indexOf(a);
    const ib = ENV_ORDER.indexOf(b);
    if (ia >= 0 && ib >= 0) return ia - ib;
    if (ia >= 0) return -1;
    if (ib >= 0) return 1;
    return a.localeCompare(b);
  });
}

export function MatrixScreen() {
  const url = useUrlState();
  const ref = url.get("ref");
  const repo = useRepoGraph(ref);
  const [hitRoots, setHitRoots] = useState<Map<string, number>>(new Map());

  return (
    <div className="app">
      <TopBar currentRef={ref}>
        <div className="spacer" />
        <SearchBox currentRef={ref} onHits={setHitRoots} />
        <RefSelect value={ref} onChange={(r) => url.set({ ref: r })} />
      </TopBar>
      {repo.isPending && <div className="center-msg">読み込み中…</div>}
      {repo.isError && <div className="error-box">読み込めませんでした: {repo.error.message}</div>}
      {repo.data && <MatrixBody graph={repo.data} currentRef={ref} hitRoots={hitRoots} />}
    </div>
  );
}

function MatrixBody({ graph, currentRef, hitRoots }: { graph: RepoGraph; currentRef: string; hitRoots: Map<string, number> }) {
  const { envs, components, cells, others } = useMemo(() => {
    const tagged = graph.roots.filter((r) => r.env && r.component);
    const envs = sortEnvs([...new Set(tagged.map((r) => r.env!))]);
    const components = [...new Set(tagged.map((r) => r.component!))].sort();
    const cells = new Map<string, RepoRoot[]>();
    for (const r of tagged) {
      const k = `${r.component}\u0000${r.env}`;
      cells.set(k, [...(cells.get(k) ?? []), r]);
    }
    const others = graph.roots.filter((r) => !(r.env && r.component));
    return { envs, components, cells, others };
  }, [graph]);

  const callers = useMemo(() => {
    const m = new Map<string, string[]>();
    for (const e of graph.edges) {
      if (e.kind !== "module_call") continue;
      m.set(e.to, [...(m.get(e.to) ?? []), e.from]);
    }
    return m;
  }, [graph]);

  return (
    <div className="matrix-layout">
      <main className="matrix-main">
        {graph.roots.length === 0 && (
          <div className="center-msg">
            ルートが見つかりませんでした。backend / provider ブロックを含むディレクトリがあるか、ripplegraph.yaml の roots.include を確認してください。
          </div>
        )}
        {envs.length > 0 && (
          <div className="matrix-scroll">
            <table className="matrix">
              <thead>
                <tr>
                  <th className="corner">コンポーネント ＼ 環境</th>
                  {envs.map((env) => (
                    <th key={env}>{env}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {components.map((comp) => (
                  <tr key={comp}>
                    <th className="row-head">{comp}</th>
                    {envs.map((env) => {
                      const roots = cells.get(`${comp}\u0000${env}`) ?? [];
                      return (
                        <td key={env}>
                          {roots.length === 0 ? (
                            <span className="cell-empty">—</span>
                          ) : (
                            roots.map((r) => <RootCell key={r.path} root={r} currentRef={currentRef} hits={hitRoots.get(r.path)} />)
                          )}
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {others.length > 0 && (
          <section className="others">
            <h2 className="section-title">その他のルート</h2>
            <ul>
              {others.map((r) => (
                <li key={r.path}>
                  <RootCell root={r} currentRef={currentRef} hits={hitRoots.get(r.path)} showPath />
                </li>
              ))}
            </ul>
          </section>
        )}
      </main>
      <aside className="matrix-side">
        <section>
          <h2 className="section-title">再利用モジュール（{graph.modules.length}）</h2>
          <ul className="module-list">
            {graph.modules.map((m) => (
              <li key={m.path}>
                <code>{m.path}</code>
                <div className="module-callers">← {(callers.get(m.path) ?? []).join(", ")}</div>
              </li>
            ))}
          </ul>
        </section>
        <section>
          <Diagnostics diagnostics={graph.diagnostics} />
        </section>
        <section className="sha-note mono">
          {graph.sha === "worktree" ? "ワーキングツリー" : graph.sha.slice(0, 12)}
        </section>
      </aside>
    </div>
  );
}

function RootCell({ root, currentRef, hits, showPath }: { root: RepoRoot; currentRef: string; hits?: number; showPath?: boolean }) {
  return (
    <Link href={rootHref(root.path, currentRef)} className={`root-cell${hits ? " hit" : ""}`} title={root.path}>
      {showPath && <span className="root-path mono">{root.path}</span>}
      <span className="root-count">
        <strong>{root.resourceCount}</strong> リソース
      </span>
      {hits ? <span className="badge hit-badge">検索一致 {hits}</span> : null}
    </Link>
  );
}
