"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "./api";

/** ワーキングツリーを表す SHA（Go の source.WorktreeSHA と同じ値） */
export const WORKTREE = "worktree";

export function useRefs() {
  return useQuery({ queryKey: ["refs"], queryFn: api.refs, staleTime: 30_000 });
}

/**
 * ref（ブランチ名・タグ・SHA）を、分かる範囲で SHA に解決する。
 * クエリキーに SHA を含めることで、同じコミットの結果を共有しつつ、ブランチが進んだら取り直す。
 * 空の ref はワーキングツリーで、キャッシュしない。
 */
export function useResolvedRef(ref: string): { key: string; request: string; ready: boolean } {
  const refs = useRefs();
  if (!ref) return { key: WORKTREE, request: "", ready: true };
  if (refs.isPending) return { key: ref, request: ref, ready: false };
  const hit = refs.data?.refs.find((r) => r.name === ref);
  return hit ? { key: hit.sha, request: hit.sha, ready: true } : { key: ref, request: ref, ready: true };
}

function cacheOptions(key: string) {
  // ワーキングツリーは毎回取り直す。コミットの結果は不変なので取り直さない。
  return key === WORKTREE
    ? { staleTime: 0, gcTime: 0, refetchOnWindowFocus: true }
    : { staleTime: Infinity, refetchOnWindowFocus: false };
}

export function useRepoGraph(ref: string) {
  const r = useResolvedRef(ref);
  return useQuery({
    queryKey: ["repo", r.key],
    queryFn: () => api.repo(r.request),
    enabled: r.ready,
    ...cacheOptions(r.key),
  });
}

export function useRootGraph(path: string, ref: string) {
  const r = useResolvedRef(ref);
  return useQuery({
    queryKey: ["root", r.key, path],
    queryFn: () => api.root(path, r.request),
    enabled: r.ready && path !== "",
    ...cacheOptions(r.key),
  });
}

export function useSearch(q: string, ref: string) {
  const r = useResolvedRef(ref);
  return useQuery({
    queryKey: ["search", r.key, q],
    queryFn: () => api.search(q, r.request),
    enabled: r.ready && q.trim().length >= 2,
    ...cacheOptions(r.key),
  });
}
