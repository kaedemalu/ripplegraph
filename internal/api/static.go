package api

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// staticHandler は Next.js の静的エクスポートを配信する。
// "/root" のような拡張子のないパスは "root.html" として探す。
func staticHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		for _, cand := range candidates(name) {
			if serveFile(w, r, fsys, cand, http.StatusOK) {
				return
			}
		}
		if serveFile(w, r, fsys, "404.html", http.StatusNotFound) {
			return
		}
		http.NotFound(w, r)
	})
}

func candidates(name string) []string {
	if name == "" {
		return []string{"index.html"}
	}
	return []string{name, name + ".html", name + "/index.html"}
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string, status int) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		return false
	}
	if strings.HasPrefix(name, "_next/static/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.Copy(w, f)
		return true
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		// embed.FS のファイルは更新時刻を持たないため、ゼロ値を渡して Last-Modified を付けない
		http.ServeContent(w, r, name, time.Time{}, rs)
		return true
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return false
	}
	http.ServeContent(w, r, name, time.Time{}, strings.NewReader(string(b)))
	return true
}
