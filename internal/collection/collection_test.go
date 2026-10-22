package collection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func loadSample(t *testing.T) *Collection {
	t.Helper()
	c, err := Load(context.Background(), filepath.Join("testdata", "sample"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", c.Warnings)
	}
	return c
}

func findNode(t *testing.T, c *Collection, rel string) *Node {
	t.Helper()
	var found *Node
	_ = c.Walk(func(n *Node) error {
		if n.RelPath == rel {
			found = n
		}
		return nil
	})
	if found == nil {
		t.Fatalf("node %q not found", rel)
	}
	return found
}

func TestLoadSampleTree(t *testing.T) {
	c := loadSample(t)

	if c.Config.Name != "sample" || c.Config.Type != "collection" {
		t.Fatalf("config = %+v", c.Config)
	}
	if c.Defaults == nil {
		t.Fatal("collection.bru defaults missing")
	}
	if len(c.Tree) != 1 || c.Tree[0].Name != "Users" {
		t.Fatalf("tree = %+v", c.Tree)
	}
	users := c.Tree[0]
	if !users.IsFolder() {
		t.Fatal("users must be a folder")
	}
	var names []string
	for _, child := range users.Children {
		names = append(names, child.Name)
	}
	// Order is seq then name, folders and requests together.
	if got := strings.Join(names, ","); got != "Admin,List users,Get user" {
		t.Fatalf("children = %s", got)
	}
	if got := len(c.Requests()); got != 3 {
		t.Fatalf("requests = %d, want 3 (vendor and hidden dirs are skipped)", got)
	}

	// Requests inside environments/ are never requests.
	for _, r := range c.Requests() {
		if strings.HasPrefix(r.RelPath, "environments/") {
			t.Fatalf("environment file %q loaded as a request", r.RelPath)
		}
	}
}

func TestResolvedHeadersOverrideByPosition(t *testing.T) {
	c := loadSample(t)
	node := findNode(t, c, "users/get.bru")

	headers := ResolvedHeaders(node)
	got := map[string]string{}
	var order []string
	for _, kv := range headers {
		got[strings.ToLower(kv.Name)] = kv.Value
		order = append(order, kv.Name)
	}
	if got["x-collection"] != "yes" {
		t.Fatalf("collection header missing: %v", got)
	}
	if got["x-users"] != "yes" {
		t.Fatalf("folder header missing: %v", got)
	}
	if got["accept"] != "text/plain" {
		t.Fatalf("folder header must override the collection value: %v", got)
	}
	if strings.Join(order, ",") != "X-Collection,Accept,X-Users" {
		t.Fatalf("header order = %v", order)
	}
}

func TestResolvedAuthInheritance(t *testing.T) {
	c := loadSample(t)

	get := ResolvedAuth(findNode(t, c, "users/get.bru"))
	if get == nil || get.Mode != "bearer" {
		t.Fatalf("get auth = %+v, want inherited bearer", get)
	}

	del := ResolvedAuth(findNode(t, c, "users/admin/delete.bru"))
	if del == nil || del.Mode != "none" {
		t.Fatalf("delete auth = %+v, want explicit none", del)
	}
}

func TestResolvedScriptsConcatenate(t *testing.T) {
	c := loadSample(t)
	node := findNode(t, c, "users/get.bru")

	scripts := ResolvedScripts(node)
	if !strings.Contains(scripts.PreRequest, "bru.setVar('collection', true)") {
		t.Fatalf("collection script missing: %q", scripts.PreRequest)
	}
}

func TestResolvedVarsPre(t *testing.T) {
	c := loadSample(t)
	vars := ResolvedVarsPre(findNode(t, c, "users/get.bru"))
	if len(vars) != 1 || vars[0].Name != "baseUrl" {
		t.Fatalf("vars = %+v", vars)
	}
}

func TestParseRequestModel(t *testing.T) {
	req, err := LoadRequestFile(context.Background(), filepath.Join("testdata", "sample", "users", "get.bru"))
	if err != nil {
		t.Fatalf("load request: %v", err)
	}
	if req.Name() != "Get user" {
		t.Fatalf("name = %q", req.Name())
	}
	if req.Method != "GET" {
		t.Fatalf("method = %q", req.Method)
	}
	if req.URL != "{{baseUrl}}/users/:id" {
		t.Fatalf("url = %q", req.URL)
	}
	if len(req.Params) != 1 || req.Params[0].Type != "path" || req.Params[0].Name != "id" {
		t.Fatalf("params = %+v", req.Params)
	}
	// Text blocks keep the file's relative indentation; display and execution
	// call Outdent.
	if got := Outdent(req.Docs); got != "Returns one user." {
		t.Fatalf("docs = %q", got)
	}
	if req.Meta.Seq != 2 {
		t.Fatalf("seq = %d", req.Meta.Seq)
	}
}

func TestRequestModelRoundTrip(t *testing.T) {
	path := filepath.Join("testdata", "sample", "users", "get.bru")
	req, err := LoadRequestFile(context.Background(), path)
	if err != nil {
		t.Fatalf("load request: %v", err)
	}

	printed := MarshalRequest(req)
	again, err := ParseRequest(path, req.RelPath, printed)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, printed)
	}
	if !reflect.DeepEqual(normalizeRequest(req), normalizeRequest(again)) {
		t.Fatalf("model changed across a round trip\n--- first ---\n%+v\n--- second ---\n%+v\n--- printed ---\n%s", req, again, printed)
	}
	if second := MarshalRequest(again); string(second) != string(printed) {
		t.Fatalf("marshalling is not idempotent\n--- first ---\n%s\n--- second ---\n%s", printed, second)
	}
}

// normalizeRequest clears fields the printer regenerates so a round-trip
// comparison focuses on content.
func normalizeRequest(r *Request) *Request {
	clone := *r
	clone.Path = ""
	clone.RelPath = ""
	return &clone
}

func TestFolderDefaultsRoundTrip(t *testing.T) {
	c := loadSample(t)
	printed := MarshalFolder(c.Defaults)
	if !strings.Contains(string(printed), "X-Collection: yes") {
		t.Fatalf("collection defaults lost headers:\n%s", printed)
	}
	if !strings.Contains(string(printed), "script:pre-request") {
		t.Fatalf("collection defaults lost scripts:\n%s", printed)
	}
}

func TestConfigPreservesUnknownKeys(t *testing.T) {
	c := loadSample(t)
	out, err := c.Config.Marshal()
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if !strings.Contains(string(out), `"presets"`) {
		t.Fatalf("unknown key dropped:\n%s", out)
	}
	if !strings.Contains(string(out), `"requestUrl"`) {
		t.Fatalf("nested unknown data dropped:\n%s", out)
	}
	if !strings.Contains(string(out), `"ignore"`) {
		t.Fatalf("ignore list dropped:\n%s", out)
	}
}

func TestConfigInvalidJSON(t *testing.T) {
	_, err := ParseConfig("bruno.json", []byte("{not json"))
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error = %v, want *ConfigError", err)
	}
	if cfgErr.Path != "bruno.json" {
		t.Fatalf("path = %q", cfgErr.Path)
	}
}

func TestEnvironmentResolution(t *testing.T) {
	c := loadSample(t)
	if len(c.Environments) != 2 {
		t.Fatalf("environments = %d", len(c.Environments))
	}
	byName := map[string]*Environment{}
	for _, env := range c.Environments {
		byName[env.Name] = env
	}
	local := byName["Local"]
	if local == nil {
		t.Fatal("Local environment missing")
	}
	if local.Color != "#4caf50" {
		t.Fatalf("color = %q", local.Color)
	}
	if len(local.Extends) != 1 || local.Extends[0] != "Base" {
		t.Fatalf("extends = %v", local.Extends)
	}

	vars, err := ResolveEnvironment(local, byName)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got := map[string]Variable{}
	for _, v := range vars {
		got[v.Name] = v
	}
	if got["baseUrl"].Value != "http://localhost:8080" {
		t.Fatalf("child must win: %+v", got["baseUrl"])
	}
	if got["region"].Value != "eu" {
		t.Fatalf("inherited variable missing: %+v", got["region"])
	}
	if apiKey, ok := got["apiKey"]; !ok || !apiKey.Secret || apiKey.Value != "" {
		t.Fatalf("secret variable = %+v", apiKey)
	}
}

func TestEnvironmentCycleDetected(t *testing.T) {
	byName := map[string]*Environment{
		"A": {Name: "A", Extends: []string{"B"}},
		"B": {Name: "B", Extends: []string{"A"}},
	}
	if _, err := ResolveEnvironment(byName["A"], byName); err == nil {
		t.Fatal("expected a cycle error")
	} else if !strings.Contains(err.Error(), "extends itself") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadWarnsOnBrokenRequest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bruno.json"), []byte(`{"version":"1","name":"t","type":"collection"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.bru"), []byte("meta {\n  name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.bru"), []byte("get {\n  url: https://example.com\n}\n"), 0o644); err != nil {
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

func TestLoadRejectsOversizedRequest(t *testing.T) {
	dir := t.TempDir()
	big := "get {\n  url: https://example.com\n}\n" + strings.Repeat("#", MaxFileSize)
	if err := os.WriteFile(filepath.Join(dir, "big.bru"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Warnings) != 1 || !errors.Is(c.Warnings[0], ErrTooLarge) {
		t.Fatalf("warnings = %v", c.Warnings)
	}
}

func TestLoadMissingCollection(t *testing.T) {
	_, err := Load(context.Background(), filepath.Join("testdata", "does-not-exist"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestResolveEnvironmentOwnVariableBeatsInheritedSecret(t *testing.T) {
	byName := map[string]*Environment{
		"Base": {
			Name: "Base",
			Vars: []Variable{{Name: "token", Enabled: true, Secret: true, Kind: VarString}},
		},
		"Local": {
			Name:    "Local",
			Extends: []string{"Base"},
			Vars:    []Variable{{Name: "token", Value: "plain", Enabled: true, Kind: VarString}},
		},
	}

	resolved, err := ResolveEnvironment(byName["Local"], byName)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("resolved = %+v", resolved)
	}
	// Bruno appends the environment's own variables after the inherited ones,
	// so a plain override in the child wins even over an inherited secret.
	if resolved[0].Value != "plain" || resolved[0].Secret {
		t.Fatalf("token = %+v, want the child's plain value", resolved[0])
	}
}

func TestResolveEnvironmentInheritedSecretBeatsInheritedPlain(t *testing.T) {
	byName := map[string]*Environment{
		"Base": {
			Name: "Base",
			Vars: []Variable{{Name: "token", Value: "plain", Enabled: true, Kind: VarString}},
		},
		"Shared": {
			Name: "Shared",
			Vars: []Variable{{Name: "token", Enabled: true, Secret: true, Kind: VarString}},
		},
		"Local": {
			Name:    "Local",
			Extends: []string{"Base", "Shared"},
		},
	}

	resolved, err := ResolveEnvironment(byName["Local"], byName)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 || !resolved[0].Secret {
		t.Fatalf("resolved = %+v, want the inherited secret to win", resolved)
	}
}

func TestCollectionResolveEnvironmentUsesItsOwnIndex(t *testing.T) {
	c := loadSample(t)
	var local *Environment
	for _, env := range c.Environments {
		if env.Name == "Local" {
			local = env
		}
	}
	if local == nil {
		t.Fatal("Local environment missing")
	}
	resolved, err := c.ResolveEnvironment(local)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	values := map[string]string{}
	for _, v := range resolved {
		values[v.Name] = v.Value
	}
	if values["baseUrl"] != "http://localhost:8080" || values["region"] != "eu" {
		t.Fatalf("resolved = %+v", values)
	}
	if resolved, err := c.ResolveEnvironment(nil); err != nil || resolved != nil {
		t.Fatalf("nil environment: %v, %v", resolved, err)
	}
}
