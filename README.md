# Ripplegraph

Terraform のリソース依存関係を、`.tf` ファイルの静的解析だけで可視化するツールです。

- モノレポ全体を「コンポーネント × 環境」のマトリクスで俯瞰できる
- ルート（スタック）ごとに、モジュール展開済みのリソース依存グラフを表示できる
- state・plan・`terraform` バイナリ・クラウドの認証情報は不要

`terraform plan` の代替ではなく、**レビューで見るべき場所を示すもの**という位置づけです。設計の詳細は [SPECIFICATION.md](./SPECIFICATION.md) を参照してください。

> 現在はステップ1（全体像の表示）まで実装済みです。MR の影響範囲表示（`base` / `head` の比較）はステップ2で対応予定です。

---

## クイックスタート

### 必要なもの

| ツール | バージョン |
| ------ | ---------- |
| Go     | 1.26 以上  |
| Node.js | 22 以上（npm 付属） |

### ビルドして起動する

```sh
make build
./bin/ripplegraph-server -repo /path/to/infra-monorepo -addr :8080
```

ブラウザで <http://localhost:8080> を開きます。

`make build` は、型定義の生成 → Web UI のビルド（`web/out/`）→ UI を埋め込んだ Go バイナリの生成、を順に行います。生成物は次の2つです。

| ファイル | 内容 |
| -------- | ---- |
| `bin/ripplegraph-server` | Web サーバー（UI 配信 + API）。単一バイナリで動く |
| `bin/ripplegraph` | CLI。RepoGraph / RootGraph を JSON で標準出力に出す |

### まず試してみる

同梱のサンプル（`testdata/basic`）で動作を確認できます。

```sh
make build
./bin/ripplegraph-server -repo testdata/basic
```

`testdata/basic` は git リポジトリではないため、ワーキングツリーのみ表示されます（ref の切り替えはできません）。

---

## サーバー（`ripplegraph-server`）

```sh
ripplegraph-server -repo <リポジトリのディレクトリ> [-addr :8080] [-cache-size 32]
```

| フラグ | 既定値 | 説明 |
| ------ | ------ | ---- |
| `-repo` | `.` | 対象の Terraform リポジトリ |
| `-addr` | `:8080` | 待ち受けアドレス |
| `-cache-size` | `32` | SHA ごとの解析結果をメモリに保持する件数（LRU） |

- `-repo` が git リポジトリなら、**チェックアウトせずに**任意のブランチ・タグ・コミットを表示できます。
- ref を指定しない場合は**ワーキングツリー**（未コミットの変更を含む）を読みます。ワーキングツリーはキャッシュしないので、ファイルを編集してリロードすれば反映されます。
- git リポジトリでないディレクトリも開けます（ワーキングツリーのみ）。

## CLI（`ripplegraph`）

```sh
# リポジトリ全体（RepoGraph）
ripplegraph -repo /path/to/infra-monorepo -ref main

# 1ルートの依存グラフ（RootGraph）
ripplegraph -repo /path/to/infra-monorepo -ref main -root envs/prd/network
```

| フラグ | 説明 |
| ------ | ---- |
| `-repo` | 対象の Terraform リポジトリ（既定 `.`） |
| `-ref` | ブランチ名・タグ・SHA。省略時はワーキングツリー |
| `-root` | 指定するとそのルートの RootGraph を出力。省略時は RepoGraph |

ステップ3で GitLab CI のジョブから使う想定です。

---

## 画面の使い方

### マトリクス画面（`/`）

- 行がコンポーネント、列が環境です。セルにはモジュール展開後の resource + data の数が出ます。該当するルートがないセルは「—」です。
- セルをクリックすると、そのルートのグラフ画面に移ります（選択中の ref を引き継ぎます）。
- `pathPattern` に一致しないルートは、マトリクスの下の「その他のルート」に並びます。
- 右上の ref で、ワーキングツリー・ブランチ・タグを切り替えます。
- 検索ボックスに2文字以上入れると、全ルートのリソースアドレスを部分一致で検索します。一致したルートのセルが強調され、結果をクリックするとそのノードを選択した状態でグラフ画面が開きます。
- 右パネルには、再利用モジュールとその呼び出し元、リポジトリ全体の診断情報が出ます。

### ルートグラフ画面（`/root`）

依存グラフは **「依存する側 → 依存される側」** の向き（`terraform graph` と同じ）で、左から右に並びます。

| 操作 | 内容 |
| ---- | ---- |
| ノードをクリック | 選択。関係するノードとエッジ以外が薄くなり、右パネルに詳細（定義元のファイル・行、依存先と被依存、`attr` / `port`）が出る |
| 右パネルの依存先・被依存をクリック | そのノードへズームして選択を移す（必要ならモジュールを展開し、var / local の表示を切り替える） |
| モジュールをダブルクリック、または `▸` | 展開（もう一度で折りたたみ）。入れ子のモジュールも展開できる |
| エッジにカーソルを合わせる／クリック | 参照している属性・ポート・ファイルと行を表示（クリックで固定） |
| 何もないところをクリック | 選択を解除 |
| 左下のボタン | ズームイン／アウト、全体表示 |

上部のツールバー:

- **var / local**：表示を切り替えます。初期状態は非表示で、それらを経由した依存は直接のエッジ（点線）にまとめて表示します。
- **すべて展開 / すべて折りたたむ**：モジュールの展開状態を一括で変えます。初期状態はすべて折りたたみです。
- **種別**：resource / data の種別ごとに表示・非表示を切り替えます。
- **ノードを検索**：ルート内のノードを検索します。↑↓ で選んで Enter で、そのノードへズームします。

ズーム倍率に応じて表示の詳細度が3段階で変わります。

| 段階 | 倍率 | 表示 |
| ---- | ---- | ---- |
| far | 0.4 未満 | ノードは色付きの箱のみ。モジュールは箱とリソース数。モジュール間のエッジは1本に集約 |
| mid | 0.4〜0.9 | リソース種別のアイコンと名前 |
| near | 0.9 以上 | ファイル・行、`count` / `for_each` のバッジ |

エッジの線種:

| 線 | 意味 |
| -- | ---- |
| 灰色の実線 | 通常の参照 |
| 紫の実線 | モジュールへの入力・モジュールの出力の参照 |
| 橙の破線 | `depends_on` |
| 赤の点線 | `lifecycle.replace_triggered_by` |
| 丸い点線 | 非表示の var / local を経由した依存 |

外部（registry / git ソース）のモジュールは破線の枠で表示され、中身は展開できません。

### URL の共有

画面の状態はクエリパラメータで表すので、URL を共有すれば同じ画面を開けます（モジュールの展開状態とズーム位置は含みません）。

```
/?ref=main
/root?path=envs/prd/network&ref=main
/root?path=envs/prd/network&ref=main&node=aws_instance.nat
```

---

## 設定ファイル（`ripplegraph.yaml`）

対象リポジトリの直下に `ripplegraph.yaml` を置くと、ルートの判定と env / component のタグ付けを調整できます。表示中の ref の時点のファイルが使われます。ファイルがなくても動きます。

```yaml
roots:
  include: ["envs/*/*"]   # glob。指定すると自動検出の代わりにこれに一致するディレクトリをルートにする
  exclude: []             # include / 自動検出の結果から除外する glob
pathPattern: "envs/{env}/{component}"  # ルートのパスから env / component を取り出すパターン
```

- glob はディレクトリのセグメントごとに照合します。`*` は1セグメント、`**` は0個以上のセグメントに一致します。
- `include` を指定しない場合は自動検出します。`backend` ブロックまたは `provider` ブロックを含み、どこからもモジュールとして参照されていないディレクトリがルートになります。
- `pathPattern` に一致しないルートは env / component なしとなり、マトリクスの「その他のルート」に表示されます。
- 設定ファイルが読めない場合は、診断情報を出したうえで既定の設定（自動検出）で続行します。

---

## 解析の仕様と制約

読み込むのは `.tf`（と `.tfvars`）だけで、`.terraform/` は除外します。

**ノードになるブロック**：`resource`、`data`、`module`、`variable`、`output`、`locals`（属性1つにつき1ノード）。`provider` / `terraform` / `moved` / `import` / `check` はノードになりません。

**モジュール**

- `source` が相対パス（`./`、`../`）のモジュールは、呼び出し元のルートの中に展開します。同じモジュールを複数回呼んでも ID は衝突しません（例: `envs/prd/network::module.vpc.aws_subnet.this`）。
- registry / git ソースのモジュールは、中身の見えないノードになります。
- モジュール境界は現状「粗い方式」です。外からの出力参照は `module.X` ノードへのエッジ、モジュールへの入力は `module.X` から参照先へのエッジになります（どの出力・入力かは `port` / `attr` で分かります）。

**静的解析のため判別できないもの**

- `count` / `for_each` のインスタンス展開（バッジで表示するのみ）
- 変数値による式の評価（`count = var.enable ? 1 : 0` で作られないリソースなど）
- registry / git ソースのモジュールの中身
- `*_override.tf` のマージ（診断情報で「未対応」と出し、内容は無視します）
- `.tf.json`、Terragrunt、Workspace

**診断情報**：構文エラーのあるファイルがあっても他のファイルの結果は表示し、問題は画面の「診断情報」に出します。

| コード | 内容 |
| ------ | ---- |
| `parse_error` | 構文エラー（そのファイルは読み飛ばす） |
| `unresolved_reference` | 参照先がモジュール内に見つからない |
| `external_module` | registry / git ソースのため中身を解決しない |
| `module_not_found` | モジュールのディレクトリに `.tf` がない |
| `module_cycle` | モジュールの循環参照（展開を打ち切る） |
| `unsupported_override` | override ファイル（未対応） |
| `duplicate_block` | 同じアドレスのブロックが重複している |
| `config_error` | `ripplegraph.yaml` が読めない |

---

## Docker

```sh
make docker   # docker build -t ripplegraph .
docker run --rm -p 8080:8080 -v /path/to/infra-monorepo:/repo:ro ripplegraph
```

対象リポジトリはコンテナの `/repo` にマウントします。社内 CA の証明書（PEM、拡張子 `.crt`）を `certs/` に置いてからビルドすると、イメージの信頼済み CA に追加されます（ステップ3の GitLab 連携で使います）。

---

## 開発

### Make ターゲット

| ターゲット | 内容 |
| ---------- | ---- |
| `make build` | 型生成 → Web ビルド → Go バイナリ生成 |
| `make dev` | Go サーバー（`:8080`）と `next dev`（`:3000`）を並行起動。`REPO=...` で対象を変更できる（既定 `testdata/basic`） |
| `make test` | `go test ./...` と Web の型チェック・ユニットテスト |
| `make go-test` / `make web-test` | どちらか一方だけ |
| `make vet` | `go vet ./...` |
| `make gen-types` | `internal/graph` から `web/src/types/graph.ts` を生成（tygo） |
| `make check-types` | 生成済みの型が最新か確認（CI 用。git 管理下で使う） |
| `make web-placeholder` | Web をビルドせずに Go のビルド・テストを通すための `web/out/` を用意 |
| `make clean` | ビルド成果物を削除 |

開発時は `make dev` で起動し、<http://localhost:3000> を開きます。`/api/*` は Go サーバーにプロキシされます。

```sh
make dev REPO=/path/to/infra-monorepo
```

### ディレクトリ構成

```
cmd/server/        Web サーバー
cmd/ripplegraph/   CLI
internal/graph/    スキーマ定義（TypeScript 型の生成元）
internal/parser/   1ディレクトリの HCL → ModuleGraph（純粋なロジック）
internal/repo/     ルート判定、モジュール展開、RepoGraph / RootGraph
internal/source/   入力元（ローカルの git リポジトリ）
internal/api/      HTTP API と静的ファイル配信
web/               Next.js（App Router）。静的エクスポートして Go に埋め込む
testdata/          ゴールデンテスト用のサンプル Terraform モノレポ
```

### テスト

- `testdata/basic`、`testdata/cycle` がサンプルのモノレポです。取りこぼしやすい構文を追加するときは、ここにケースを足してください。
- parser と repo の出力は、別々のゴールデンファイル（`testdata/golden/parser/`、`testdata/golden/repo/`）と比較します。意図した変更で期待値を更新するときは次を実行します。

  ```sh
  go test ./... -update
  ```

  テストを持つ Go パッケージは、`-update` フラグを受け付けるために `internal/testutil` を import する必要があります。

- Web の表示用グラフ変換（`web/src/lib/viewGraph.ts`）は vitest でテストしています。

### スキーマを変更するとき

`internal/graph/graph.go` が正です。変更したら `make gen-types` で `web/src/types/graph.ts` を生成し直してください（生成ファイルは手で編集しないこと）。enum 相当の定数は、TypeScript の union 型として生成されるように型名を接頭辞に付けます（例: `NodeKindResource`）。

### HTTP API

| メソッド・パス | パラメータ | 返り値 |
| -------------- | ---------- | ------ |
| `GET /api/repo` | `ref`（省略時はワーキングツリー） | `RepoGraph` |
| `GET /api/root` | `path`（必須）、`ref` | `RootGraph` |
| `GET /api/refs` | なし | リポジトリ名、HEAD、ブランチ・タグの一覧 |
| `GET /api/search` | `q`、`ref`、`limit`（既定 200、最大 1000） | 全ルートの resource / data / module の検索結果 |

エラーは `{ "error": "..." }` の JSON で返します。`base` / `head` の指定（影響範囲モード）は現在 501 を返します。
