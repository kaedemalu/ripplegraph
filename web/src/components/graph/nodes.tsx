"use client";

import { Handle, Position, type Node as FlowNode, type NodeProps } from "@xyflow/react";
import { memo } from "react";
import { resourceIcon } from "@/lib/resourceIcon";
import { addressOf } from "@/lib/urls";
import type { ViewNode } from "@/lib/viewGraph";
import { useGraphActions } from "./actions";
import { useZoomStage } from "./zoom";

export type GraphNodeData = { view: ViewNode };
export type GraphFlowNode = FlowNode<GraphNodeData>;

/** 親モジュールのアドレスを取り除いた、モジュール内のローカルアドレス */
export function localAddress(v: ViewNode): string {
  const a = v.node.address;
  if (!v.node.parentModule) return a;
  const prefix = addressOf(v.node.parentModule) + ".";
  return a.startsWith(prefix) ? a.slice(prefix.length) : a;
}

/** 表示名（resource / data は名前、それ以外はローカルアドレス） */
function displayName(v: ViewNode): string {
  const local = localAddress(v);
  if (v.node.kind === "resource" || v.node.kind === "data") return local.split(".").pop() ?? local;
  return local;
}

const Handles = () => (
  <>
    <Handle type="target" position={Position.Left} isConnectable={false} />
    <Handle type="source" position={Position.Right} isConnectable={false} />
  </>
);

function Badges({ v }: { v: ViewNode }) {
  const m = v.node.meta;
  return (
    <span className="node-badges">
      {m?.count && <span className="badge">count</span>}
      {m?.forEach && <span className="badge">for_each</span>}
    </span>
  );
}

function ResourceNodeImpl({ data }: NodeProps<GraphFlowNode>) {
  const stage = useZoomStage();
  const v = data.view;
  const n = v.node;
  const icon = resourceIcon(n.type);
  return (
    <div className={`gnode kind-${n.kind} stage-${stage}`}>
      <Handles />
      {stage !== "far" && (
        <>
          <span className="gnode-icon" style={{ background: icon.color }} title={n.type}>
            {n.kind === "data" ? "D" : icon.label}
          </span>
          <span className="gnode-body">
            <span className="gnode-type">
              {n.kind === "data" ? "data." : ""}
              {n.type}
            </span>
            <span className="gnode-name">{displayName(v)}</span>
            {stage === "near" && (
              <span className="gnode-loc">
                {n.file}:{n.line}
              </span>
            )}
          </span>
          {stage === "near" ? <Badges v={v} /> : (n.meta?.count || n.meta?.forEach) && <span className="gnode-multi" title="count / for_each" />}
        </>
      )}
    </div>
  );
}

const KIND_LABEL: Record<string, string> = { variable: "var", local: "local", output: "output" };

function ValueNodeImpl({ data }: NodeProps<GraphFlowNode>) {
  const stage = useZoomStage();
  const v = data.view;
  const n = v.node;
  return (
    <div className={`gnode kind-${n.kind} stage-${stage}`}>
      <Handles />
      {stage !== "far" && (
        <>
          <span className="gnode-icon">{KIND_LABEL[n.kind] ?? n.kind}</span>
          <span className="gnode-body">
            <span className="gnode-name">{displayName(v)}</span>
            {stage === "near" && (
              <span className="gnode-loc">
                {n.file}:{n.line}
              </span>
            )}
          </span>
        </>
      )}
    </div>
  );
}

function ModuleNodeImpl({ id, data }: NodeProps<GraphFlowNode>) {
  const stage = useZoomStage();
  const { toggleModule } = useGraphActions();
  const v = data.view;
  const n = v.node;
  return (
    <div className={`gnode kind-module stage-${stage}${n.external ? " external" : ""}`}>
      <Handles />
      <span className="gnode-icon">{n.external ? "ext" : "mod"}</span>
      <span className="gnode-body">
        {stage !== "far" && <span className="gnode-type">{n.external ? "外部モジュール" : "module"}</span>}
        <span className="gnode-name">{displayName(v)}</span>
        {stage === "near" && (
          <span className="gnode-loc">
            {n.file}:{n.line}
          </span>
        )}
      </span>
      {stage === "near" && <Badges v={v} />}
      {v.expandable && (
        <button
          className="gnode-toggle nodrag"
          title="展開する（ダブルクリックでも可）"
          onClick={(e) => {
            e.stopPropagation();
            toggleModule(id);
          }}
        >
          ▸ {v.resourceCount}
        </button>
      )}
    </div>
  );
}

function GroupNodeImpl({ id, data }: NodeProps<GraphFlowNode>) {
  const stage = useZoomStage();
  const { toggleModule } = useGraphActions();
  const v = data.view;
  return (
    <div className={`ggroup stage-${stage}`}>
      <Handles />
      <div className="ggroup-header">
        <button
          className="gnode-toggle nodrag"
          title="折りたたむ"
          onClick={(e) => {
            e.stopPropagation();
            toggleModule(id);
          }}
        >
          ▾
        </button>
        <span className="ggroup-name">{displayName(v)}</span>
        <span className="ggroup-count">{v.resourceCount} リソース</span>
        {stage === "near" && <Badges v={v} />}
      </div>
      {stage === "far" && (
        <div className="ggroup-far">
          <span>{displayName(v)}</span>
          <strong>{v.resourceCount}</strong>
        </div>
      )}
    </div>
  );
}

// nodeTypes はコンポーネント外で定義する（毎レンダリングで作り直さない）
export const nodeTypes = {
  resource: memo(ResourceNodeImpl),
  value: memo(ValueNodeImpl),
  module: memo(ModuleNodeImpl),
  group: memo(GroupNodeImpl),
};

export function nodeTypeOf(v: ViewNode): keyof typeof nodeTypes {
  if (v.isGroup) return "group";
  switch (v.node.kind) {
    case "module":
      return "module";
    case "variable":
    case "local":
    case "output":
      return "value";
    default:
      return "resource";
  }
}
