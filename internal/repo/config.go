package repo

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"ripplegraph/internal/graph"
)

// ConfigFile はリポジトリ直下の設定ファイル名。
const ConfigFile = "ripplegraph.yaml"

// Config は ripplegraph.yaml の内容。
type Config struct {
	Roots struct {
		// glob。指定時は自動検出より優先する。"**" は0個以上のディレクトリに一致する。
		Include []string `yaml:"include"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"roots"`
	// ルートに env / component タグを付けるためのパターン（例: "envs/{env}/{component}"）
	PathPattern string `yaml:"pathPattern"`
}

func parseConfig(b []byte) (Config, []graph.Diagnostic) {
	var c Config
	if b == nil {
		return c, nil
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, []graph.Diagnostic{{
			Severity: graph.SeverityError, Code: graph.DiagCodeConfigError,
			Message: fmt.Sprintf("%s を読めません（既定の設定で続行します）: %v", ConfigFile, err),
			File:    ConfigFile,
		}}
	}
	var diags []graph.Diagnostic
	for _, g := range append(append([]string{}, c.Roots.Include...), c.Roots.Exclude...) {
		if _, err := path.Match(strings.ReplaceAll(g, "**", "*"), ""); err != nil {
			diags = append(diags, graph.Diagnostic{
				Severity: graph.SeverityError, Code: graph.DiagCodeConfigError,
				Message: fmt.Sprintf("glob %q が不正です: %v", g, err), File: ConfigFile,
			})
		}
	}
	return c, diags
}

// matchGlob はディレクトリのパスが glob に一致するかを判定する。
// セグメントごとに path.Match で照合し、"**" は0個以上のセグメントに一致する。
func matchGlob(pattern, dir string) bool {
	return matchSegs(splitPath(pattern), splitPath(dir))
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return nil
	}
	return strings.Split(p, "/")
}

func matchSegs(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegs(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	return err == nil && ok && matchSegs(pat[1:], segs[1:])
}

// matchPathPattern は "envs/{env}/{component}" のようなパターンでパスからタグを取り出す。
func matchPathPattern(pattern, dir string) (map[string]string, bool) {
	if pattern == "" {
		return nil, false
	}
	pat, segs := splitPath(pattern), splitPath(dir)
	if len(pat) != len(segs) {
		return nil, false
	}
	tags := map[string]string{}
	for i, p := range pat {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			tags[p[1:len(p)-1]] = segs[i]
			continue
		}
		if ok, err := path.Match(p, segs[i]); err != nil || !ok {
			return nil, false
		}
	}
	return tags, true
}
