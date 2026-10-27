// Package graph はバックエンドとフロントエンドで共有するグラフのスキーマを定義する。
// web/src/types/graph.ts はこのパッケージから tygo で生成する（make gen-types）。
package graph

// NodeKind はノードの種類。
type NodeKind string

const (
	NodeKindResource NodeKind = "resource"
	NodeKindData     NodeKind = "data"
	NodeKindModule   NodeKind = "module"
	NodeKindVariable NodeKind = "variable"
	NodeKindLocal    NodeKind = "local"
	NodeKindOutput   NodeKind = "output"
)

// EdgeKind は RootGraph のエッジの種類。
type EdgeKind string

const (
	EdgeKindReference          EdgeKind = "reference"
	EdgeKindDependsOn          EdgeKind = "depends_on"
	EdgeKindReplaceTriggeredBy EdgeKind = "replace_triggered_by"
	EdgeKindModuleIO           EdgeKind = "module_io"
)

// ChangeKind は影響範囲モードでのノードの状態（ステップ2）。
type ChangeKind string

const (
	ChangeKindAdded    ChangeKind = "added"
	ChangeKindRemoved  ChangeKind = "removed"
	ChangeKindModified ChangeKind = "modified"
	ChangeKindAffected ChangeKind = "affected"
)

// Severity は診断情報の重要度。
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// DiagCode は診断情報のコード。
type DiagCode string

const (
	DiagCodeParseError          DiagCode = "parse_error"
	DiagCodeUnresolvedReference DiagCode = "unresolved_reference"
	DiagCodeExternalModule      DiagCode = "external_module"
	DiagCodeUnsupportedOverride DiagCode = "unsupported_override"
	DiagCodeDuplicateBlock      DiagCode = "duplicate_block"
	DiagCodeModuleNotFound      DiagCode = "module_not_found"
	DiagCodeModuleCycle         DiagCode = "module_cycle"
	DiagCodeConfigError         DiagCode = "config_error"
)

// RepoEdgeKind は RepoGraph のエッジの種類。
type RepoEdgeKind string

const (
	RepoEdgeKindModuleCall  RepoEdgeKind = "module_call"
	RepoEdgeKindRemoteState RepoEdgeKind = "remote_state"
)

// NodeMeta はノードの補足情報。
type NodeMeta struct {
	Count   bool `json:"count,omitempty"`
	ForEach bool `json:"forEach,omitempty"`
}

// Node は RootGraph のノード。
type Node struct {
	// "envs/prd/network::module.vpc.aws_subnet.private"
	ID      string   `json:"id"`
	Kind    NodeKind `json:"kind"`
	Address string   `json:"address"`
	// "aws_subnet"（resource / data のみ）
	Type string `json:"type,omitempty"`
	// 親 module ノードの id（グループ化用）
	ParentModule string `json:"parentModule,omitempty"`
	// 中身の見えないモジュール（registry / git ソース）
	External bool       `json:"external,omitempty"`
	File     string     `json:"file"`
	Line     int        `json:"line"`
	Meta     *NodeMeta  `json:"meta,omitempty"`
	Change   ChangeKind `json:"change,omitempty"`
}

// Edge は「依存する側（From）→ 依存される側（To）」の向きのエッジ。
type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
	// 参照が書かれている属性名。ネストしている場合は "ingress.cidr_blocks" のようにドットでつなぐ。
	Attr string `json:"attr,omitempty"`
	// モジュール境界での出力名・入力名。
	Port   string     `json:"port,omitempty"`
	File   string     `json:"file"`
	Line   int        `json:"line"`
	Change ChangeKind `json:"change,omitempty" tstype:"\"added\" | \"removed\""`
}

// Diagnostic はパースエラーや未解決の参照などの診断情報。
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     DiagCode `json:"code"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
}

// RootGraph は1ルートのリソース単位の依存グラフ（モジュール展開済み）。
type RootGraph struct {
	// 影響範囲モードでは head の SHA
	SHA         string       `json:"sha"`
	BaseSHA     string       `json:"baseSha,omitempty"`
	Root        string       `json:"root"`
	Env         string       `json:"env,omitempty"`
	Component   string       `json:"component,omitempty"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// RepoRoot は RepoGraph 上のルート。
type RepoRoot struct {
	Path      string `json:"path"`
	Env       string `json:"env,omitempty"`
	Component string `json:"component,omitempty"`
	// モジュール展開後の resource + data の数
	ResourceCount int        `json:"resourceCount"`
	Change        ChangeKind `json:"change,omitempty" tstype:"\"modified\" | \"affected\""`
}

// RepoModule は RepoGraph 上の再利用モジュール。
type RepoModule struct {
	Path   string     `json:"path"`
	Change ChangeKind `json:"change,omitempty" tstype:"\"modified\""`
}

// RepoEdge はルート・モジュール間のエッジ。From / To はルートまたはモジュールのパス。
type RepoEdge struct {
	From string       `json:"from"`
	To   string       `json:"to"`
	Kind RepoEdgeKind `json:"kind"`
}

// RepoGraph はリポジトリ全体のグラフ。
type RepoGraph struct {
	SHA         string       `json:"sha"`
	BaseSHA     string       `json:"baseSha,omitempty"`
	Roots       []RepoRoot   `json:"roots"`
	Modules     []RepoModule `json:"modules"`
	Edges       []RepoEdge   `json:"edges"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Ref は UI の ref 選択用のブランチ・タグ。
type Ref struct {
	Name string `json:"name"`
	Kind string `json:"kind" tstype:"\"branch\" | \"tag\""`
	SHA  string `json:"sha"`
}

// RefsResponse は GET /api/refs の返り値。
type RefsResponse struct {
	// リポジトリ名（ディレクトリ名）
	Repo string `json:"repo"`
	// HEAD が指すブランチ名（detached や git 管理外では空）
	Head string `json:"head,omitempty"`
	// git リポジトリでない場合は false（ワーキングツリーのみ閲覧可能）
	Git  bool  `json:"git"`
	Refs []Ref `json:"refs"`
}

// SearchHit は GET /api/search の1件。
type SearchHit struct {
	Root    string   `json:"root"`
	NodeID  string   `json:"nodeId"`
	Address string   `json:"address"`
	Kind    NodeKind `json:"kind"`
	Type    string   `json:"type,omitempty"`
}

// SearchResponse は GET /api/search の返り値。
type SearchResponse struct {
	SHA  string      `json:"sha"`
	Hits []SearchHit `json:"hits"`
	// 件数上限で打ち切った場合 true
	Truncated bool `json:"truncated"`
}

// ErrorResponse は API のエラー応答。
type ErrorResponse struct {
	Error string `json:"error"`
}
