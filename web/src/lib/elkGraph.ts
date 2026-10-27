/** 表示用グラフと ELK のグラフ表現の相互変換（純粋関数）。 */
import type { ElkExtendedEdge, ElkNode } from "elkjs/lib/elk-api";
import { GROUP_HEADER, type LayoutPosition, type LayoutRequest } from "./layoutTypes";

const rootOptions: Record<string, string> = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT",
  "elk.hierarchyHandling": "INCLUDE_CHILDREN",
  "elk.layered.spacing.nodeNodeBetweenLayers": "80",
  "elk.spacing.nodeNode": "24",
  "elk.spacing.edgeNode": "16",
  "elk.layered.nodePlacement.strategy": "BRANDES_KOEPF",
  "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
  "elk.separateConnectedComponents": "true",
  "elk.spacing.componentComponent": "60",
};

const groupOptions: Record<string, string> = {
  "elk.padding": `[top=${GROUP_HEADER + 12},left=16,bottom=16,right=16]`,
};

/** モジュールを入れ子のグループにした ELK のグラフを作る。 */
export function toElkGraph(req: LayoutRequest): ElkNode {
  const elkNodes = new Map<string, ElkNode>();
  for (const n of req.nodes) {
    elkNodes.set(n.id, n.isGroup ? { id: n.id, children: [], layoutOptions: groupOptions } : { id: n.id, width: n.width, height: n.height });
  }
  const root: ElkNode = { id: "__root__", layoutOptions: rootOptions, children: [] };
  for (const n of req.nodes) {
    const parent = n.parent ? elkNodes.get(n.parent) : undefined;
    (parent ?? root).children!.push(elkNodes.get(n.id)!);
  }
  const edges: ElkExtendedEdge[] = req.edges
    .filter((e) => elkNodes.has(e.source) && elkNodes.has(e.target))
    .map((e) => ({ id: e.id, sources: [e.source], targets: [e.target] }));
  root.edges = edges;
  return root;
}

/** ELK の結果から各ノードの座標（親からの相対座標）を取り出す。 */
export function positionsOf(result: ElkNode): Record<string, LayoutPosition> {
  const positions: Record<string, LayoutPosition> = {};
  const walk = (n: ElkNode) => {
    for (const c of n.children ?? []) {
      positions[c.id] = { x: c.x ?? 0, y: c.y ?? 0, width: c.width ?? 0, height: c.height ?? 0 };
      walk(c);
    }
  };
  walk(result);
  return positions;
}
