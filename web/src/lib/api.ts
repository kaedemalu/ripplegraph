import type { RefsResponse, RepoGraph, RootGraph, SearchResponse } from "@/types/graph";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function getJSON<T>(path: string, params: Record<string, string | undefined>): Promise<T> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v) qs.set(k, v);
  }
  const url = qs.size > 0 ? `${path}?${qs}` : path;
  const res = await fetch(url);
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // JSON でないエラー応答
    }
    throw new ApiError(message, res.status);
  }
  return (await res.json()) as T;
}

export const api = {
  refs: () => getJSON<RefsResponse>("/api/refs", {}),
  repo: (ref: string) => getJSON<RepoGraph>("/api/repo", { ref }),
  root: (path: string, ref: string) => getJSON<RootGraph>("/api/root", { path, ref }),
  search: (q: string, ref: string) => getJSON<SearchResponse>("/api/search", { q, ref }),
};
