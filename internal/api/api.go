// Package api は HTTP API と、埋め込んだ静的ファイル（Web UI）の配信を行う。
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strconv"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"

	"ripplegraph/internal/graph"
	"ripplegraph/internal/repo"
	"ripplegraph/internal/source"
)

// Options はサーバーの設定。
type Options struct {
	// CacheSize は SHA ごとの解析結果を保持する件数（既定 32）。
	CacheSize int
	// Static は Web UI の静的ファイル。nil なら配信しない。
	Static fs.FS
}

// Server は API サーバー。
type Server struct {
	src    source.Source
	cache  *lru.Cache[string, *repo.Repo]
	group  singleflight.Group
	static fs.FS
}

// New はサーバーを作る。
func New(src source.Source, opts Options) *Server {
	if opts.CacheSize <= 0 {
		opts.CacheSize = 32
	}
	cache, _ := lru.New[string, *repo.Repo](opts.CacheSize)
	return &Server{src: src, cache: cache, static: opts.Static}
}

// Handler は HTTP ハンドラを返す。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repo", s.handleRepo)
	mux.HandleFunc("GET /api/root", s.handleRoot)
	mux.HandleFunc("GET /api/refs", s.handleRefs)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "API が見つかりません: "+r.URL.Path)
	})
	if s.static != nil {
		mux.Handle("/", staticHandler(s.static))
	}
	return mux
}

// load は ref を解決し、その SHA の Repo を返す。ワーキングツリーはキャッシュしない。
func (s *Server) load(ref string) (*repo.Repo, error) {
	sha, err := s.src.Resolve(ref)
	if err != nil {
		return nil, err
	}
	if sha == source.WorktreeSHA {
		files, err := s.src.Files(sha)
		if err != nil {
			return nil, err
		}
		return repo.Load(sha, files), nil
	}
	if r, ok := s.cache.Get(sha); ok {
		return r, nil
	}
	v, err, _ := s.group.Do(sha, func() (any, error) {
		if r, ok := s.cache.Get(sha); ok {
			return r, nil
		}
		files, err := s.src.Files(sha)
		if err != nil {
			return nil, err
		}
		r := repo.Load(sha, files)
		s.cache.Add(sha, r)
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*repo.Repo), nil
}

// loadFromQuery はクエリの ref（または base / head）から Repo を読む。
// 影響範囲モード（base / head）はステップ2で対応する。
func (s *Server) loadFromQuery(w http.ResponseWriter, r *http.Request) (*repo.Repo, bool) {
	q := r.URL.Query()
	if q.Get("base") != "" || q.Get("head") != "" {
		writeError(w, http.StatusNotImplemented, "影響範囲モード（base / head）は未対応です")
		return nil, false
	}
	rp, err := s.load(q.Get("ref"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, source.ErrRefNotFound) {
			status = http.StatusNotFound
		} else {
			log.Printf("load %q: %v", q.Get("ref"), err)
		}
		writeError(w, status, err.Error())
		return nil, false
	}
	return rp, true
}

func (s *Server) handleRepo(w http.ResponseWriter, r *http.Request) {
	rp, ok := s.loadFromQuery(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, rp.RepoGraph())
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path は必須です")
		return
	}
	rp, ok := s.loadFromQuery(w, r)
	if !ok {
		return
	}
	rg, ok := rp.RootGraph(path)
	if !ok {
		writeError(w, http.StatusNotFound, "ルートが見つかりません: "+path)
		return
	}
	writeJSON(w, http.StatusOK, rg)
}

func (s *Server) handleRefs(w http.ResponseWriter, r *http.Request) {
	lister, ok := s.src.(source.RefLister)
	if !ok {
		writeJSON(w, http.StatusOK, graph.RefsResponse{Refs: []graph.Ref{}})
		return
	}
	refs, err := lister.Refs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, refs)
}

const defaultSearchLimit = 200

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := defaultSearchLimit
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 1000 {
		limit = l
	}
	rp, ok := s.loadFromQuery(w, r)
	if !ok {
		return
	}
	hits, truncated := rp.Search(q.Get("q"), limit)
	writeJSON(w, http.StatusOK, graph.SearchResponse{SHA: rp.SHA(), Hits: hits, Truncated: truncated})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, graph.ErrorResponse{Error: msg})
}
