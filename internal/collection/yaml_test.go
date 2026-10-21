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

// writeYAMLTree creates a collection tree from path -> content.
func writeYAMLTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestYAMLAlternativeExtension(t *testing.T) {
	dir := writeYAMLTree(t, map[string]string{
		"opencollection.yaml":     "opencollection: '1.0.0'\ninfo:\n  name: alt\n",
		"users/folder.yaml":       "info:\n  name: Users\n  type: folder\n",
		"users/one.yaml":          "info:\n  name: One\n  type: http\nhttp:\n  url: https://a\n",
		"environments/Local.yaml": "name: Local\nvariables:\n  - name: a\n    value: b\n",
	})

	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Format != FormatYAML {
		t.Fatalf("format = %v", c.Format)
	}
	if len(c.Requests()) != 1 {
		t.Fatalf("requests = %d", len(c.Requests()))
	}
	if len(c.Environments) != 1 || c.Environments[0].Name != "Local" {
		t.Fatalf("environments = %+v", c.Environments)
	}
	var folder *Node
	_ = c.Walk(func(n *Node) error {
		if n.IsFolder() {
			folder = n
		}
		return nil
	})
	if folder == nil || folder.Name != "Users" {
		t.Fatalf("folder = %+v", folder)
	}
}

func TestLoadYAMLSkipsNonRequestItems(t *testing.T) {
	dir := writeYAMLTree(t, map[string]string{
		"opencollection.yml": "opencollection: '1.0.0'\ninfo:\n  name: items\n",
		"real.yml":           "info:\n  name: real\n  type: http\nhttp:\n  url: https://a\n",
		"tool.yml":           "info:\n  name: tool\n  type: script\nscript:\n  code: |-\n    console.log(1)\n",
	})

	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Requests()) != 1 {
		t.Fatalf("requests = %d, want only the http item", len(c.Requests()))
	}
	if len(c.Warnings) != 1 || !strings.Contains(c.Warnings[0].Error(), "not shown as requests") {
		t.Fatalf("warnings = %v", c.Warnings)
	}
}

func TestYAMLUnknownKeysStayInTheDocument(t *testing.T) {
	src := []byte(`info:
  name: future
  type: http
  x-new-info: keep
http:
  method: GET
  url: https://example.com
x-future-block:
  a: 1
`)
	req, err := ParseYAMLRequest("future.yml", "future.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Source == nil {
		t.Fatal("request must keep its source document")
	}
	out, err := req.Source.Bytes()
	if err != nil {
		t.Fatalf("bytes: %v", err)
	}
	for _, want := range []string{"x-new-info: keep", "x-future-block:", "a: 1"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("document lost %q:\n%s", want, out)
		}
	}
}

func TestYAMLBodyVariants(t *testing.T) {
	src := []byte(`info:
  name: bodies
  type: http
http:
  method: POST
  url: https://example.com
  body:
    type: form-urlencoded
    data:
      - name: a
        value: '1'
      - name: b
        value: '2'
        disabled: true
`)
	req, err := ParseYAMLRequest("bodies.yml", "bodies.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Body.Mode != BodyFormURLEncoded {
		t.Fatalf("mode = %q", req.Body.Mode)
	}
	if len(req.Body.Form) != 2 || req.Body.Form[0].Name != "a" {
		t.Fatalf("form = %+v", req.Body.Form)
	}
	if req.Body.Form[1].Enabled {
		t.Fatalf("disabled form field = %+v", req.Body.Form[1])
	}

	multipart := []byte(`info:
  name: upload
  type: http
http:
  url: https://example.com
  body:
    type: multipart-form
    data:
      - name: note
        value: hello
      - name: file
        files: [/tmp/a.txt]
        contentType: text/plain
`)
	req, err = ParseYAMLRequest("upload.yml", "upload.yml", multipart)
	if err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	if req.Body.Mode != BodyMultipart || len(req.Body.Form) != 2 {
		t.Fatalf("multipart = %+v", req.Body)
	}
	if req.Body.Form[1].Type != "file" || len(req.Body.Form[1].Files) != 1 {
		t.Fatalf("file field = %+v", req.Body.Form[1])
	}
	if req.Body.Form[1].ContentType != "text/plain" {
		t.Fatalf("content type = %q", req.Body.Form[1].ContentType)
	}

	fileBody := []byte(`info:
  name: binary
  type: http
http:
  url: https://example.com
  body:
    type: file
    data:
      - path: /tmp/payload.bin
        contentType: application/octet-stream
`)
	req, err = ParseYAMLRequest("binary.yml", "binary.yml", fileBody)
	if err != nil {
		t.Fatalf("parse file body: %v", err)
	}
	if req.Body.Mode != BodyFile || len(req.Body.Files) != 1 || req.Body.Files[0].Path != "/tmp/payload.bin" {
		t.Fatalf("file body = %+v", req.Body)
	}
}

func TestYAMLScriptsAndActions(t *testing.T) {
	src := []byte(`info:
  name: scripts
  type: http
http:
  url: https://example.com
runtime:
  scripts:
    - type: before-request
      code: 'pre()'
    - type: after-response
      code: 'post()'
    - type: tests
      code: 'test()'
    - type: grpc:before-call-start
      code: 'grpc()'
  actions:
    - type: set-variable
      selector: { expression: res.body.id, method: jsonq }
      variable: { name: userId, scope: request }
`)
	req, err := ParseYAMLRequest("scripts.yml", "scripts.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Scripts.PreRequest != "pre()" || req.Scripts.PostResponse != "post()" {
		t.Fatalf("scripts = %+v", req.Scripts)
	}
	if req.Tests != "test()" {
		t.Fatalf("tests = %q", req.Tests)
	}
	if req.Scripts.GRPC["before-call-start"] != "grpc()" {
		t.Fatalf("grpc scripts = %+v", req.Scripts.GRPC)
	}
	if len(req.VarsPost) != 1 || req.VarsPost[0].Name != "userId" {
		t.Fatalf("post vars = %+v", req.VarsPost)
	}
	if !req.VarsPost[0].Local {
		t.Fatalf("scope: request must map to a local variable: %+v", req.VarsPost[0])
	}
	if req.VarsPost[0].Value != "res.body.id" {
		t.Fatalf("value = %q", req.VarsPost[0].Value)
	}
}

func TestYAMLWebsocketTypeIsNormalised(t *testing.T) {
	src := []byte("info:\n  name: socket\n  type: websocket\nwebsocket:\n  url: wss://example.com\n")
	req, err := ParseYAMLRequest("socket.yml", "socket.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Meta.Type != string(TypeWS) {
		t.Fatalf("type = %q, want %q", req.Meta.Type, TypeWS)
	}
	if req.URL != "wss://example.com" {
		t.Fatalf("url = %q", req.URL)
	}
}

func TestYAMLCollectionIgnoreAndActions(t *testing.T) {
	dir := writeYAMLTree(t, map[string]string{
		"opencollection.yml": `opencollection: '1.0.0'
info:
  name: ignored
request:
  actions:
    - type: set-variable
      selector: { expression: res.body.token, method: jsonq }
      variable: { name: token, scope: runtime }
extensions:
  bruno:
    ignore:
      - vendor
`,
		"a.yml":        "info:\n  name: a\n  type: http\nhttp:\n  url: https://a\n",
		"vendor/b.yml": "info:\n  name: b\n  type: http\nhttp:\n  url: https://b\n",
	})

	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Requests()) != 1 {
		t.Fatalf("requests = %d, want vendor ignored", len(c.Requests()))
	}
	if c.Defaults == nil || len(c.Defaults.VarsPost) != 1 {
		t.Fatalf("collection actions = %+v", c.Defaults)
	}
	node := findNode(t, c, "a.yml")
	vars := ResolvedVarsPost(node)
	if len(vars) != 1 || vars[0].Name != "token" {
		t.Fatalf("resolved post vars = %+v", vars)
	}
}

func TestYAMLDescriptionAndEnvironmentExtras(t *testing.T) {
	src := []byte(`info:
  name: described
  type: http
  description: 'A described request'
http:
  url: https://a
`)
	req, err := ParseYAMLRequest("d.yml", "d.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Meta.Description != "A described request" {
		t.Fatalf("description = %q", req.Meta.Description)
	}

	envSrc := []byte(`name: Staging
extends: [Base, Shared]
color: '#ff0000'
variables:
  - name: retries
    value: { type: number, data: '3' }
externalSecrets:
  type: vault
  variables:
    - name: apiKey
`)
	env, err := ParseYAMLEnvironment("Staging.yml", "Staging", envSrc)
	if err != nil {
		t.Fatalf("parse env: %v", err)
	}
	if len(env.Extends) != 2 || env.Extends[1] != "Shared" {
		t.Fatalf("extends = %v", env.Extends)
	}
	if env.Color != "#ff0000" {
		t.Fatalf("color = %q", env.Color)
	}
	if env.Vars[0].Kind != VarNumber || env.Vars[0].Value != "3" {
		t.Fatalf("typed value = %+v", env.Vars[0])
	}
	if len(env.External) != 1 || env.External[0].Provider != "vault" {
		t.Fatalf("external = %+v", env.External)
	}
	if len(env.External[0].Pairs) != 1 || env.External[0].Pairs[0].Name != "apiKey" {
		t.Fatalf("external pairs = %+v", env.External[0].Pairs)
	}
}
