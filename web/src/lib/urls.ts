/** 画面の状態はすべてクエリパラメータで表す（静的エクスポートで動的ルートが使えないため）。 */

export function matrixHref(ref: string): string {
  return ref ? `/?${new URLSearchParams({ ref })}` : "/";
}

export function rootHref(path: string, ref: string, node?: string): string {
  const qs = new URLSearchParams({ path });
  if (ref) qs.set("ref", ref);
  if (node) qs.set("node", node);
  return `/root?${qs}`;
}

/** ノード ID（"<ルート>::<アドレス>"）からアドレスを取り出す。 */
export function addressOf(id: string): string {
  const i = id.indexOf("::");
  return i < 0 ? id : id.slice(i + 2);
}
