package collection

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func yamlFixture(parts ...string) string {
	return filepath.Join(append([]string{"testdata", "bruno-yaml"}, parts...)...)
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return src
}

func TestDetectFormat(t *testing.T) {
	bru := t.TempDir()
	if err := os.WriteFile(filepath.Join(bru, ConfigFile), []byte(`{"version":"1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectFormat(bru); got != FormatBRU {
		t.Fatalf("bruno.json dir = %v", got)
	}

	yml := t.TempDir()
	if err := os.WriteFile(filepath.Join(yml, OpenCollectionFile), []byte("opencollection: '1.0.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectFormat(yml); got != FormatYAML {
		t.Fatalf("opencollection.yml dir = %v", got)
	}

	// Both markers: OpenCollection wins, matching Bruno.
	if err := os.WriteFile(filepath.Join(yml, ConfigFile), []byte(`{"version":"1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectFormat(yml); got != FormatYAML {
		t.Fatalf("both formats = %v, want yaml", got)
	}
}

func TestParseYAMLRequestHTTP(t *testing.T) {
	path := yamlFixture("edgegrid", "EdgeGrid POST JSON 200.yml")
	req, err := ParseYAMLRequest(path, "EdgeGrid POST JSON 200.yml", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if req.Name() != "EdgeGrid POST JSON 200" {
		t.Fatalf("name = %q", req.Name())
	}
	if req.Meta.Type != "http" || req.Meta.Seq != 3 {
		t.Fatalf("meta = %+v", req.Meta)
	}
	if req.Method != "POST" {
		t.Fatalf("method = %q", req.Method)
	}
	if !strings.Contains(req.URL, "/api/auth/edgegrid/resource") {
		t.Fatalf("url = %q", req.URL)
	}

	if req.Auth == nil || req.Auth.Mode != "akamai-edgegrid" {
		t.Fatalf("auth = %+v", req.Auth)
	}
	params := map[string]string{}
	for _, kv := range req.Auth.Params {
		params[kv.Name] = kv.Value
	}
	if params["accessToken"] != "akab-access-token-bruno-test" {
		t.Fatalf("auth params = %+v", params)
	}
	if _, ok := params["type"]; ok {
		t.Fatal("auth type must be the mode, not a param")
	}

	if req.Body.Mode != BodyJSON {
		t.Fatalf("body mode = %q", req.Body.Mode)
	}
	if !strings.Contains(req.Body.Text, `"name":"bruno"`) {
		t.Fatalf("body text = %q", req.Body.Text)
	}

	if len(req.Assertions) != 2 {
		t.Fatalf("assertions = %+v", req.Assertions)
	}
	if req.Assertions[0].Expression != "res.status" || req.Assertions[0].Operator != "eq" || req.Assertions[0].Operand != "200" {
		t.Fatalf("assertion 0 = %+v", req.Assertions[0])
	}

	if req.Settings.MaxRedirects == nil || *req.Settings.MaxRedirects != 5 {
		t.Fatalf("maxRedirects = %+v", req.Settings.MaxRedirects)
	}
	if req.Settings.FollowRedirects == nil || !*req.Settings.FollowRedirects {
		t.Fatalf("followRedirects = %+v", req.Settings.FollowRedirects)
	}
}

func TestParseYAMLRequestGraphQL(t *testing.T) {
	path := yamlFixture("graphql", "test-graphql.yml")
	req, err := ParseYAMLRequest(path, "test-graphql.yml", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Meta.Type != "graphql" {
		t.Fatalf("type = %q", req.Meta.Type)
	}
	if req.Method != "POST" {
		t.Fatalf("method = %q", req.Method)
	}
	if req.Body.Mode != BodyGraphQL {
		t.Fatalf("body mode = %q", req.Body.Mode)
	}
	if req.Body.GraphQLVars != "{}" {
		t.Fatalf("graphql vars = %q", req.Body.GraphQLVars)
	}
	if req.Auth == nil || req.Auth.Mode != "inherit" {
		t.Fatalf("auth = %+v (bare `auth: inherit` must parse as a mode)", req.Auth)
	}
}

func TestParseYAMLCollectionDefaults(t *testing.T) {
	path := yamlFixture("descriptions", OpenCollectionFile)
	cfg, defaults, err := ParseYAMLCollection(path, readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Name != "col-description-yml" {
		t.Fatalf("name = %q", cfg.Name)
	}
	if defaults == nil || len(defaults.Headers) != 3 {
		t.Fatalf("headers = %+v", defaults)
	}
	if defaults.Headers[0].Name != "X-Version" || defaults.Headers[0].Value != "2.0" {
		t.Fatalf("header 0 = %+v", defaults.Headers[0])
	}
	if defaults.Headers[0].Description != "Single-line header desc" {
		t.Fatalf("header description = %q", defaults.Headers[0].Description)
	}
	if !strings.Contains(defaults.Headers[1].Description, "Header line one") {
		t.Fatalf("multiline description = %q", defaults.Headers[1].Description)
	}
	if len(defaults.VarsPre) != 3 {
		t.Fatalf("vars = %+v", defaults.VarsPre)
	}
	if defaults.VarsPre[0].Name != "baseUrl" || defaults.VarsPre[0].Value != "https://example.com" {
		t.Fatalf("var 0 = %+v", defaults.VarsPre[0])
	}
}

func TestParseYAMLFolder(t *testing.T) {
	path := yamlFixture("folders", "async-folder", YAMLFolderFile)
	folder, err := ParseYAMLFolder(path, "async-folder", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if folder.Meta.Name != "async-folder" {
		t.Fatalf("name = %q", folder.Meta.Name)
	}
	if folder.Meta.Seq != 8 {
		t.Fatalf("seq = %d", folder.Meta.Seq)
	}
	if !strings.Contains(folder.Tests, "folder-level sync test") {
		t.Fatalf("folder tests = %q", folder.Tests)
	}
}

func TestParseYAMLEnvironment(t *testing.T) {
	path := yamlFixture("edgegrid", "environments", "Local.yml")
	env, err := ParseYAMLEnvironment(path, "Local", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env.Name != "Local" {
		t.Fatalf("name = %q", env.Name)
	}
	if len(env.Vars) != 1 || env.Vars[0].Name != "localhost" {
		t.Fatalf("vars = %+v", env.Vars)
	}
	if env.Vars[0].Value != "http://localhost:8081" {
		t.Fatalf("value = %q", env.Vars[0].Value)
	}
}

func TestParseYAMLEnvironmentSecrets(t *testing.T) {
	src := []byte("name: Prod\nvariables:\n  - name: token\n    secret: true\n  - name: color\n    value: blue\n")
	env, err := ParseYAMLEnvironment("Prod.yml", "Prod", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(env.SecretNames) != 1 || env.SecretNames[0] != "token" {
		t.Fatalf("secrets = %v", env.SecretNames)
	}
	for _, v := range env.Vars {
		if v.Name == "token" {
			if !v.Secret || v.Value != "" {
				t.Fatalf("secret variable = %+v", v)
			}
		}
	}
}

func TestParseYAMLRequestUnknownKeysSurvive(t *testing.T) {
	src := []byte(`info:
  name: future
  type: http
  x-new-info: keep
http:
  method: GET
  url: https://example.com
  x-new-http: keep
x-future-block:
  a: 1
`)
	req, err := ParseYAMLRequest("future.yml", "future.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Method != "GET" {
		t.Fatalf("method = %q", req.Method)
	}
	// The reader must accept the file; unknown keys stay in the document the
	// writer patches in M1-14.
	if len(req.Meta.Extra) != 0 {
		t.Fatalf("YAML extras are not model pairs: %+v", req.Meta.Extra)
	}
}

func TestLoadYAMLCollection(t *testing.T) {
	c, err := Load(context.Background(), yamlFixture("edgegrid"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Format != FormatYAML {
		t.Fatalf("format = %v", c.Format)
	}
	if c.Config.Name != "edgegrid-testbench-yml" {
		t.Fatalf("name = %q", c.Config.Name)
	}
	if c.Defaults == nil {
		t.Fatal("collection defaults missing")
	}
	if c.Defaults.Auth == nil || c.Defaults.Auth.Mode != "akamai-edgegrid" {
		t.Fatalf("collection auth = %+v", c.Defaults.Auth)
	}

	requests := c.Requests()
	if len(requests) != 7 {
		names := make([]string, 0, len(requests))
		for _, req := range requests {
			names = append(names, req.RelPath)
		}
		t.Fatalf("requests = %d (%v)", len(requests), names)
	}
	if len(c.Environments) != 1 || c.Environments[0].Name != "Local" {
		t.Fatalf("environments = %+v", c.Environments)
	}
	if len(c.Warnings) != 0 {
		t.Fatalf("warnings = %v", c.Warnings)
	}

	// Inheritance uses the collection-level auth.
	node := findNode(t, c, "EdgeGrid GET 200.yml")
	auth := ResolvedAuth(node)
	if auth == nil || auth.Mode != "akamai-edgegrid" {
		t.Fatalf("resolved auth = %+v", auth)
	}
}

func TestLoadYAMLFolders(t *testing.T) {
	c, err := Load(context.Background(), yamlFixture("folders"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Format != FormatYAML {
		t.Fatalf("format = %v", c.Format)
	}

	var folder *Node
	_ = c.Walk(func(n *Node) error {
		if n.IsFolder() && n.Name == "async-folder" {
			folder = n
		}
		return nil
	})
	if folder == nil {
		t.Fatalf("async-folder not found in tree")
	}
	if folder.Folder.Meta.Seq != 8 {
		t.Fatalf("folder seq = %d", folder.Folder.Meta.Seq)
	}
	if len(folder.Children) != 1 {
		t.Fatalf("folder children = %d", len(folder.Children))
	}

	// bruno.json also exists in this fixture; YAML wins and the user is warned.
	found := false
	for _, warning := range c.Warnings {
		if strings.Contains(warning.Error(), "both exist") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a dual-format warning, got %v", c.Warnings)
	}
}

func TestLoadYAMLRejectsBrokenItem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, OpenCollectionFile), []byte("opencollection: '1.0.0'\ninfo:\n  name: broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("info:\n  name: [oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.yml"), []byte("info:\n  name: ok\n  type: http\nhttp:\n  url: https://example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Warnings) != 1 {
		t.Fatalf("warnings = %v", c.Warnings)
	}
	if len(c.Requests()) != 1 {
		t.Fatalf("the healthy request must still load, got %d", len(c.Requests()))
	}
}
