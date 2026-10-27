import type { Metadata } from "next";
import { Providers } from "./providers";
import "@xyflow/react/dist/style.css";
import "./globals.css";
import "./graph.css";
import "./matrix.css";

export const metadata: Metadata = {
  title: "Ripplegraph",
  description: "Terraform のリソース依存関係の可視化",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
