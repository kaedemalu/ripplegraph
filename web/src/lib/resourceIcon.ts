/** リソース種別（aws_security_group など）から、表示用の短いラベルと色を作る。 */

const PROVIDER_COLORS: Record<string, string> = {
  aws: "#e8871e",
  google: "#4285f4",
  azurerm: "#0078d4",
  azuread: "#0078d4",
  kubernetes: "#326ce5",
  helm: "#0f1689",
  github: "#6e5494",
  gitlab: "#fc6d26",
  cloudflare: "#f38020",
  datadog: "#632ca6",
  random: "#7a8599",
  null: "#7a8599",
  tls: "#7a8599",
  terraform: "#7b42bc",
};

export type ResourceIcon = { provider: string; label: string; color: string };

export function resourceIcon(type: string | undefined): ResourceIcon {
  if (!type) return { provider: "", label: "?", color: "#7a8599" };
  const [provider, ...rest] = type.split("_");
  const words = rest.length > 0 ? rest : [provider];
  const label =
    words.length === 1 ? words[0].slice(0, 2).toUpperCase() : words.slice(0, 3).map((w) => w[0]?.toUpperCase() ?? "").join("");
  return { provider, label, color: PROVIDER_COLORS[provider] ?? hashColor(provider) };
}

function hashColor(s: string): string {
  let h = 0;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return `hsl(${h % 360} 45% 50%)`;
}
