"use client";

import { useStore, type ReactFlowState } from "@xyflow/react";

/** セマンティックズームの段階 */
export type ZoomStage = "far" | "mid" | "near";

export function stageOf(zoom: number): ZoomStage {
  if (zoom < 0.4) return "far";
  if (zoom < 0.9) return "mid";
  return "near";
}

const selectStage = (s: ReactFlowState) => stageOf(s.transform[2]);

/**
 * 丸めた段階だけを購読する。セレクタはズームのたびに走るが、
 * 段階が変わらない限り再レンダリングされない。
 */
export function useZoomStage(): ZoomStage {
  return useStore(selectStage);
}
