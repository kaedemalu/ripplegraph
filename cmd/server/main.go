// Command server は Ripplegraph の Web サーバー（UI 配信 + API）。
//
//	ripplegraph-server --repo /path/to/infra-monorepo --addr :8080
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"ripplegraph/internal/api"
	"ripplegraph/internal/source"
	"ripplegraph/web"
)

func main() {
	repoDir := flag.String("repo", ".", "対象の Terraform リポジトリのディレクトリ")
	addr := flag.String("addr", ":8080", "待ち受けアドレス")
	cacheSize := flag.Int("cache-size", 32, "SHA ごとの解析結果を保持する件数")
	flag.Parse()

	src, err := source.NewLocalGit(*repoDir)
	if err != nil {
		log.Fatalf("リポジトリを開けません: %v", err)
	}
	srv := api.New(src, api.Options{CacheSize: *cacheSize, Static: web.FS()})

	hs := &http.Server{
		Addr:              *addr,
		Handler:           logRequests(srv.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("ripplegraph: %s を http://%s で配信します", src.Dir(), displayAddr(*addr))
	log.Fatal(hs.ListenAndServe())
}

func displayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "localhost" + addr
	}
	return addr
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			log.Printf("%s %s (%s)", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
		}
	})
}
