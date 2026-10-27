"use client";

import { useEffect, useState } from "react";
import ElkConstructor, { type ELK } from "elkjs/lib/elk-api";
import { positionsOf, toElkGraph } from "./elkGraph";
import { NODE_HEIGHT, NODE_WIDTH, type LayoutPosition, type LayoutRequest } from "./layoutTypes";
import type { ViewGraph } from "./viewGraph";

export type Layout = { positions: Record<string, LayoutPosition>; elapsedMs: number };

let elk: ELK | null = null;
// elk-api は worker のエラーを拾わないため、ここで受けて計算中の要求を失敗させる
const workerErrorListeners = new Set<(err: Error) => void>();

function getElk(): ELK {
  // レイアウト計算は Web Worker（layout.worker.ts）上で行い、メインスレッドは結果を受け取るだけにする。
  // Turbopack の worker ブートストラップは importScripts を使うため、module worker にはしない。
  elk ??= new ElkConstructor({
    workerFactory: () => {
      const w = new Worker(new URL("./layout.worker.ts", import.meta.url));
      w.addEventListener("error", (ev) => {
        const err = new Error(ev.message || "レイアウト用の Web Worker でエラーが発生しました");
        for (const l of workerErrorListeners) l(err);
        // 次の要求では worker を作り直す
        elk?.terminateWorker();
        elk = null;
      });
      return w;
    },
  });
  return elk;
}

function withWorkerErrors<T>(p: Promise<T>): Promise<T> {
  let listener: (err: Error) => void = () => {};
  const failed = new Promise<never>((_, reject) => {
    listener = reject;
    workerErrorListeners.add(listener);
  });
  return Promise.race([p, failed]).finally(() => workerErrorListeners.delete(listener));
}

// 「SHA + ルート + 表示オプション + 展開状態」をキーにしたレイアウトのキャッシュ
const MAX_CACHE = 50;
const cache = new Map<string, Layout>();
const inflight = new Map<string, Promise<Layout>>();

function requestLayout(key: string, view: ViewGraph): Promise<Layout> {
  const cached = cache.get(key);
  if (cached) return Promise.resolve(cached);
  const running = inflight.get(key);
  if (running) return running;

  const req: LayoutRequest = {
    nodes: view.nodes.map((n) => ({ id: n.id, parent: n.parent, width: NODE_WIDTH, height: NODE_HEIGHT, isGroup: n.isGroup })),
    edges: view.edges.map((e) => ({ id: e.id, source: e.source, target: e.target })),
  };
  const start = performance.now();
  const promise = withWorkerErrors(getElk().layout(toElkGraph(req)))
    .then((result) => {
      const l: Layout = { positions: positionsOf(result), elapsedMs: performance.now() - start };
      cache.set(key, l);
      if (cache.size > MAX_CACHE) cache.delete(cache.keys().next().value!);
      return l;
    })
    .finally(() => inflight.delete(key));
  inflight.set(key, promise);
  return promise;
}

type LayoutState = { key: string | null; view: ViewGraph | null; layout: Layout | null; error: string | null };

/**
 * 表示用グラフのレイアウトを Web Worker で計算する。
 * 計算中は、直前に計算できたレイアウトとその表示用グラフの組を返し続ける（UI を固めず、表示も崩さない）。
 */
export function useLayout(key: string | null, view: ViewGraph | null) {
  const [state, setState] = useState<LayoutState>({ key: null, view: null, layout: null, error: null });

  useEffect(() => {
    if (!key || !view) return;
    let cancelled = false;
    requestLayout(key, view).then(
      (layout) => !cancelled && setState({ key, view, layout, error: null }),
      (err: Error) => !cancelled && setState((s) => ({ ...s, error: err.message })),
    );
    return () => {
      cancelled = true;
    };
  }, [key, view]);

  return {
    /** layout と対応する表示用グラフ */
    view: state.view,
    layout: state.layout,
    layoutKey: state.key,
    loading: key !== state.key && !state.error,
    error: state.error,
  };
}
