/** レイアウト計算（Web Worker）の入出力。 */

export type LayoutNodeInput = {
  id: string;
  parent?: string;
  width: number;
  height: number;
  isGroup: boolean;
};

export type LayoutRequest = {
  nodes: LayoutNodeInput[];
  edges: { id: string; source: string; target: string }[];
};

export type LayoutPosition = {
  /** 親グループからの相対座標（React Flow の parentId と同じ扱い） */
  x: number;
  y: number;
  width: number;
  height: number;
};

export const NODE_WIDTH = 240;
export const NODE_HEIGHT = 64;
/** グループの見出しの高さ */
export const GROUP_HEADER = 40;
