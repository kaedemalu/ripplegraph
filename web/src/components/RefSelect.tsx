"use client";

import { useRefs } from "@/lib/queries";

/** ref の選択。空文字はワーキングツリー。 */
export function RefSelect({ value, onChange }: { value: string; onChange: (ref: string) => void }) {
  const refs = useRefs();
  const list = refs.data?.refs ?? [];
  const branches = list.filter((r) => r.kind === "branch");
  const tags = list.filter((r) => r.kind === "tag");
  const known = value === "" || list.some((r) => r.name === value);

  return (
    <label className="field">
      ref
      <select className="select" value={value} onChange={(e) => onChange(e.target.value)} disabled={refs.isPending}>
        <option value="">ワーキングツリー（未コミットの変更を含む）</option>
        {!known && <option value={value}>{value}</option>}
        {branches.length > 0 && (
          <optgroup label="ブランチ">
            {branches.map((r) => (
              <option key={r.name} value={r.name}>
                {r.name}
                {r.name === refs.data?.head ? "（HEAD）" : ""}
              </option>
            ))}
          </optgroup>
        )}
        {tags.length > 0 && (
          <optgroup label="タグ">
            {tags.map((r) => (
              <option key={r.name} value={r.name}>
                {r.name}
              </option>
            ))}
          </optgroup>
        )}
      </select>
    </label>
  );
}
