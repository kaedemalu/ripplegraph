import type { NextConfig } from "next";
import { PHASE_PRODUCTION_BUILD } from "next/constants";

// 本番ビルドは静的エクスポートして Go バイナリに埋め込む。
// 開発時（next dev）は /api/* を Go サーバーへプロキシする。
const apiOrigin = process.env.RIPPLEGRAPH_API ?? "http://localhost:8080";

export default function config(phase: string): NextConfig {
  const base: NextConfig = {
    images: { unoptimized: true },
    reactStrictMode: true,
  };
  if (phase === PHASE_PRODUCTION_BUILD) {
    return { ...base, output: "export" };
  }
  return {
    ...base,
    async rewrites() {
      return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
    },
  };
}
