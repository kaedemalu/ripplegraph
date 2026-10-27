GO      ?= go
NPM     ?= npm
BIN_DIR ?= bin
# make dev で読み込む Terraform リポジトリ
REPO    ?= testdata/basic
ADDR    ?= :8080

export NEXT_TELEMETRY_DISABLED := 1

.PHONY: all gen-types check-types web-deps web-build build web-placeholder test go-test web-test vet dev docker clean

all: build

## internal/graph から web/src/types/graph.ts を生成する
gen-types:
	$(GO) tool tygo generate

## CI 用: 生成済みの型が Go のスキーマと一致しているかを確認する
check-types: gen-types
	@git diff --exit-code -- web/src/types/graph.ts || (echo "web/src/types/graph.ts が古くなっています。make gen-types を実行してコミットしてください" && exit 1)

web/node_modules: web/package.json web/package-lock.json
	cd web && $(NPM) ci
	@touch web/node_modules

web-deps: web/node_modules

## gen-types の後に next build（web/out/ に出力）
web-build: gen-types web-deps
	cd web && $(NPM) run build
	@# next build が web/out を作り直すので、コミット対象の .gitkeep を戻す（clone 直後でも go:embed が通るように）
	@touch web/out/.gitkeep

## web-build の後に go build（web/out/ を埋め込んだバイナリを生成）
build: web-build
	$(GO) build -trimpath -o $(BIN_DIR)/ripplegraph-server ./cmd/server
	$(GO) build -trimpath -o $(BIN_DIR)/ripplegraph ./cmd/ripplegraph
	@echo "生成しました: $(BIN_DIR)/ripplegraph-server, $(BIN_DIR)/ripplegraph"

## web をビルドせずに go test / go vet を通すための web/out/ を作る（ビルド済みなら何もしない）
web-placeholder:
	@mkdir -p web/out
	@touch web/out/.gitkeep
	@test -f web/out/index.html || echo '<!doctype html><meta charset="utf-8"><title>Ripplegraph</title><p>UI がビルドされていません。<code>make build</code> を実行してください。</p>' > web/out/index.html

go-test: web-placeholder
	$(GO) test ./...

web-test: web-deps
	cd web && $(NPM) run typecheck && $(NPM) test

## go test ./... と web のユニットテスト
test: go-test web-test

vet: web-placeholder
	$(GO) vet ./...

## Go サーバーと next dev を並行起動する（UI は http://localhost:3000）
dev: web-placeholder web-deps
	@trap 'kill 0' INT TERM EXIT; \
	$(GO) run ./cmd/server -repo $(REPO) -addr $(ADDR) & \
	(cd web && RIPPLEGRAPH_API=http://localhost$(ADDR) $(NPM) run dev) & \
	wait

docker:
	docker build -t ripplegraph .

clean:
	rm -rf $(BIN_DIR) web/.next
	find web/out -mindepth 1 ! -name .gitkeep -delete 2>/dev/null || true
