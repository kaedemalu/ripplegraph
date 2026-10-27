import type { Diagnostic } from "@/types/graph";

const SEV_LABEL: Record<string, string> = { error: "エラー", warning: "警告", info: "情報" };

export function Diagnostics({ diagnostics, open }: { diagnostics: Diagnostic[]; open?: boolean }) {
  if (diagnostics.length === 0) return null;
  const count = (s: string) => diagnostics.filter((d) => d.severity === s).length;
  const summary = (["error", "warning", "info"] as const)
    .filter((s) => count(s) > 0)
    .map((s) => `${SEV_LABEL[s]} ${count(s)}`)
    .join(" / ");
  return (
    <details className="diags" open={open}>
      <summary>診断情報（{summary}）</summary>
      <ul>
        {diagnostics.map((d, i) => (
          <li key={i}>
            <span className={`sev-${d.severity}`}>{SEV_LABEL[d.severity] ?? d.severity}</span>
            <span>
              {d.message}
              {d.file && (
                <div className="diag-loc">
                  {d.file}
                  {d.line ? `:${d.line}` : ""} · {d.code}
                </div>
              )}
            </span>
          </li>
        ))}
      </ul>
    </details>
  );
}
