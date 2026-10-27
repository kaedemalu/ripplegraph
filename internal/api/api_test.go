package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"ripplegraph/internal/graph"
	"ripplegraph/internal/source"
	"ripplegraph/internal/testutil"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	src, err := source.NewLocalGit(testutil.TestdataDir("basic"))
	if err != nil {
		t.Fatal(err)
	}
	static := fstest.MapFS{
		"index.html":                 {Data: []byte("index")},
		"root.html":                  {Data: []byte("root page")},
		"404.html":                   {Data: []byte("not found page")},
		"_next/static/chunks/app.js": {Data: []byte("js")},
	}
	ts := httptest.NewServer(New(src, Options{Static: static}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, url string) (int, string, http.Header) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header
}

func TestAPI(t *testing.T) {
	ts := newTestServer(t)

	t.Run("repo", func(t *testing.T) {
		status, body, _ := get(t, ts.URL+"/api/repo")
		if status != 200 {
			t.Fatalf("status %d: %s", status, body)
		}
		var g graph.RepoGraph
		if err := json.Unmarshal([]byte(body), &g); err != nil {
			t.Fatal(err)
		}
		if g.SHA != source.WorktreeSHA || len(g.Roots) != 5 {
			t.Errorf("repo graph: sha=%s roots=%d", g.SHA, len(g.Roots))
		}
	})

	t.Run("root", func(t *testing.T) {
		status, body, _ := get(t, ts.URL+"/api/root?path=envs/prd/network")
		if status != 200 {
			t.Fatalf("status %d: %s", status, body)
		}
		var g graph.RootGraph
		if err := json.Unmarshal([]byte(body), &g); err != nil {
			t.Fatal(err)
		}
		if g.Root != "envs/prd/network" || g.Env != "prd" || len(g.Nodes) == 0 {
			t.Errorf("root graph: %+v", g)
		}
	})

	errorCases := []struct {
		name, path string
		status     int
	}{
		{"path なし", "/api/root", 400},
		{"ルートでない", "/api/root?path=modules/vpc", 404},
		{"ref が存在しない", "/api/repo?ref=no-such-ref", 404},
		{"影響範囲モード", "/api/repo?base=main&head=feature", 501},
		{"未知の API", "/api/unknown", 404},
	}
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			status, body, _ := get(t, ts.URL+c.path)
			if status != c.status {
				t.Errorf("status = %d, want %d", status, c.status)
			}
			var e graph.ErrorResponse
			if err := json.Unmarshal([]byte(body), &e); err != nil || e.Error == "" {
				t.Errorf("JSON のエラーではありません: %s", body)
			}
		})
	}

	t.Run("refs", func(t *testing.T) {
		status, body, _ := get(t, ts.URL+"/api/refs")
		if status != 200 || !strings.Contains(body, `"repo":"basic"`) {
			t.Errorf("status %d: %s", status, body)
		}
	})

	t.Run("search", func(t *testing.T) {
		status, body, _ := get(t, ts.URL+"/api/search?q=aws_vpc.this")
		var res graph.SearchResponse
		if err := json.Unmarshal([]byte(body), &res); err != nil || status != 200 || len(res.Hits) != 3 {
			t.Errorf("status %d: %s", status, body)
		}
	})
}

func TestStatic(t *testing.T) {
	ts := newTestServer(t)
	cases := []struct {
		path   string
		status int
		body   string
	}{
		{"/", 200, "index"},
		{"/root", 200, "root page"},
		{"/root?path=envs/prd/network", 200, "root page"},
		{"/root.html", 200, "root page"},
		{"/_next/static/chunks/app.js", 200, "js"},
		{"/nope", 404, "not found page"},
	}
	for _, c := range cases {
		status, body, _ := get(t, ts.URL+c.path)
		if status != c.status || body != c.body {
			t.Errorf("%s: status=%d body=%q", c.path, status, body)
		}
	}
	_, _, h := get(t, ts.URL+"/_next/static/chunks/app.js")
	if !strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Errorf("Cache-Control = %s", h.Get("Cache-Control"))
	}
}
