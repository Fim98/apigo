package collection

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requestSignature renders the fields the writer owns, so a parse → write →
// parse cycle can be compared without depending on formatting.
func requestSignature(r *Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "info %q type=%s seq=%d tags=%v desc=%q\n", r.Meta.Name, r.Meta.Type, r.Meta.Seq, r.Meta.Tags, r.Meta.Description)
	fmt.Fprintf(&b, "req %s %s\n", r.Method, r.URL)
	for _, kv := range r.Headers {
		fmt.Fprintf(&b, "h %q=%q enabled=%v desc=%q\n", kv.Name, kv.Value, kv.Enabled, kv.Description)
	}
	for _, p := range r.Params {
		fmt.Fprintf(&b, "p %q=%q type=%s enabled=%v\n", p.Name, p.Value, p.Type, p.Enabled)
	}
	for _, kv := range r.Metadata {
		fmt.Fprintf(&b, "m %q=%q\n", kv.Name, kv.Value)
	}
	if r.Auth == nil {
		b.WriteString("auth nil\n")
	} else {
		fmt.Fprintf(&b, "auth %s\n", r.Auth.Mode)
		for _, kv := range r.Auth.Params {
			fmt.Fprintf(&b, "  a %q=%q\n", kv.Name, kv.Value)
		}
	}
	fmt.Fprintf(&b, "body %s text=%q vars=%q files=%v\n", r.Body.Mode, r.Body.Text, r.Body.GraphQLVars, r.Body.Files)
	for _, f := range r.Body.Form {
		fmt.Fprintf(&b, "  f %q=%q type=%s files=%v ct=%q enabled=%v\n", f.Name, f.Value, f.Type, f.Files, f.ContentType, f.Enabled)
	}
	for _, v := range r.VarsPre {
		fmt.Fprintf(&b, "pre %q=%q kind=%s enabled=%v desc=%q\n", v.Name, v.Value, v.Kind, v.Enabled, v.Description)
	}
	for _, v := range r.VarsPost {
		fmt.Fprintf(&b, "post %q=%q local=%v enabled=%v\n", v.Name, v.Value, v.Local, v.Enabled)
	}
	for _, a := range r.Assertions {
		fmt.Fprintf(&b, "assert %q %s %q enabled=%v\n", a.Expression, a.Operator, a.Operand, a.Enabled)
	}
	fmt.Fprintf(&b, "scripts pre=%q post=%q tests=%q grpc=%v\n", r.Scripts.PreRequest, r.Scripts.PostResponse, r.Tests, r.Scripts.GRPC)
	fmt.Fprintf(&b, "settings encode=%v follow=%v max=%v timeout=%d fwd=%v omit=%v\n",
		r.Settings.EncodeURL, derefBool(r.Settings.FollowRedirects), derefInt(r.Settings.MaxRedirects),
		r.Settings.TimeoutMS, derefBool(r.Settings.ForwardAuthorizationHeader), r.Settings.OmitHeaders)
	fmt.Fprintf(&b, "docs %q\n", r.Docs)
	return b.String()
}

func derefBool(v *bool) any {
	if v == nil {
		return "nil"
	}
	return *v
}

func derefInt(v *int) any {
	if v == nil {
		return "nil"
	}
	return *v
}

func TestMarshalYAMLRoundTripsFixtures(t *testing.T) {
	roots, err := filepath.Glob(filepath.Join("testdata", "bruno-yaml", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) == 0 {
		t.Fatal("no YAML fixtures")
	}

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		c, err := Load(context.Background(), root)
		if err != nil {
			t.Fatalf("%s: load: %v", root, err)
		}
		for _, req := range c.Requests() {
			out, err := MarshalYAMLRequest(req)
			if err != nil {
				t.Fatalf("%s: marshal: %v", req.RelPath, err)
			}
			again, err := ParseYAMLRequest(req.Path, req.RelPath, out)
			if err != nil {
				t.Fatalf("%s: reparse: %v\n%s", req.RelPath, err, out)
			}
			if got, want := requestSignature(again), requestSignature(req); got != want {
				t.Fatalf("%s: round trip changed the model\n--- got ---\n%s\n--- want ---\n%s\n--- yaml ---\n%s",
					req.RelPath, got, want, out)
			}
			// A second write must be stable.
			out2, err := MarshalYAMLRequest(again)
			if err != nil {
				t.Fatalf("%s: second marshal: %v", req.RelPath, err)
			}
			if !bytes.Equal(out, out2) {
				t.Fatalf("%s: writing is not idempotent\n--- first ---\n%s\n--- second ---\n%s", req.RelPath, out, out2)
			}
			if bytes.Contains(out, []byte("\t")) {
				t.Fatalf("%s: output uses tabs", req.RelPath)
			}
		}
	}
}

func TestMarshalYAMLRequestPreservesUnknownKeys(t *testing.T) {
	src := []byte(`info:
  name: keep me
  type: http
  x-new-info: keep
http:
  method: GET
  url: https://example.com
x-future-block:
  a: 1
`)
	req, err := ParseYAMLRequest("keep.yml", "keep.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	req.URL = "https://changed.example.com"

	out, err := MarshalYAMLRequest(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"x-new-info: keep", "x-future-block:", "url: https://changed.example.com"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("output lost %q:\n%s", want, out)
		}
	}
}

func TestMarshalYAMLRequestEdits(t *testing.T) {
	path := yamlFixture("edgegrid", "EdgeGrid POST JSON 200.yml")
	req, err := ParseYAMLRequest(path, "EdgeGrid POST JSON 200.yml", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	req.Meta.Name = "Renamed"
	req.URL = "https://example.com/edited"
	req.Headers = append(req.Headers, KeyValue{Name: "X-New", Value: "1", Enabled: true})
	req.Auth.Params = append(req.Auth.Params, KeyValue{Name: "baseURL", Value: "https://api.example.com", Enabled: true})

	out, err := MarshalYAMLRequest(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	again, err := ParseYAMLRequest(path, req.RelPath, out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	if again.Meta.Name != "Renamed" || again.URL != "https://example.com/edited" {
		t.Fatalf("edits lost: %+v %q", again.Meta, again.URL)
	}
	if len(again.Headers) != 1 || again.Headers[0].Name != "X-New" {
		t.Fatalf("headers = %+v", again.Headers)
	}
	if again.Auth == nil || again.Auth.Mode != "akamai-edgegrid" {
		t.Fatalf("auth = %+v", again.Auth)
	}
	var baseURL string
	for _, kv := range again.Auth.Params {
		if kv.Name == "baseURL" {
			baseURL = kv.Value
		}
	}
	if baseURL != "https://api.example.com" {
		t.Fatalf("auth param not written: %+v", again.Auth.Params)
	}
	// Untouched assertions survive the patch.
	if len(again.Assertions) != 2 {
		t.Fatalf("assertions = %+v", again.Assertions)
	}
}

func TestMarshalYAMLRequestFromScratch(t *testing.T) {
	req := &Request{
		Meta:   Meta{Name: "New request", Type: string(TypeHTTP), Seq: 1},
		Method: "POST",
		URL:    "https://api.example.com/items",
		Headers: []KeyValue{
			{Name: "Content-Type", Value: "application/json", Enabled: true},
			{Name: "X-Debug", Value: "1", Enabled: false},
		},
		Params: []Param{{KeyValue: KeyValue{Name: "id", Value: "42", Enabled: true}, Type: "path"}},
		Auth:   &Auth{Mode: "bearer", Params: []KeyValue{{Name: "token", Value: "{{token}}", Enabled: true}}},
		Body:   Body{Mode: BodyJSON, Text: "{\"a\": 1}"},
		VarsPost: []Variable{
			{Name: "id", Value: "res.body.id", Enabled: true, Kind: VarString},
		},
		Assertions: []Assertion{{Expression: "res.status", Operator: "eq", Operand: "201", Enabled: true}},
		Scripts:    Scripts{PreRequest: "console.log(1);"},
		Tests:      "test('ok', () => {});",
		Settings:   Settings{EncodeURL: true},
	}

	out, err := MarshalYAMLRequest(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	again, err := ParseYAMLRequest("new.yml", "new.yml", out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	if again.Method != "POST" || again.URL != req.URL {
		t.Fatalf("method/url = %s %s", again.Method, again.URL)
	}
	if len(again.Headers) != 2 || again.Headers[1].Enabled {
		t.Fatalf("headers = %+v", again.Headers)
	}
	if len(again.Params) != 1 || again.Params[0].Type != "path" {
		t.Fatalf("params = %+v", again.Params)
	}
	if again.Auth == nil || again.Auth.Mode != "bearer" {
		t.Fatalf("auth = %+v", again.Auth)
	}
	if again.Body.Mode != BodyJSON || again.Body.Text != req.Body.Text {
		t.Fatalf("body = %+v", again.Body)
	}
	if len(again.VarsPost) != 1 || again.VarsPost[0].Value != "res.body.id" {
		t.Fatalf("post vars = %+v", again.VarsPost)
	}
	if len(again.Assertions) != 1 || again.Assertions[0].Operand != "201" {
		t.Fatalf("assertions = %+v", again.Assertions)
	}
	if again.Scripts.PreRequest == "" || again.Tests == "" {
		t.Fatalf("scripts = %+v tests=%q", again.Scripts, again.Tests)
	}
}

func TestMarshalYAMLFolderAndCollection(t *testing.T) {
	path := yamlFixture("folders", "async-folder", YAMLFolderFile)
	folder, err := ParseYAMLFolder(path, "async-folder", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse folder: %v", err)
	}
	folder.Meta.Name = "renamed-folder"
	out, err := MarshalYAMLFolder(folder)
	if err != nil {
		t.Fatalf("marshal folder: %v", err)
	}
	again, err := ParseYAMLFolder(path, "async-folder", out)
	if err != nil {
		t.Fatalf("reparse folder: %v\n%s", err, out)
	}
	if again.Meta.Name != "renamed-folder" {
		t.Fatalf("folder name = %q", again.Meta.Name)
	}
	if !strings.Contains(again.Tests, "folder-level sync test") {
		t.Fatalf("folder tests lost:\n%s", out)
	}

	c, err := Load(context.Background(), yamlFixture("edgegrid"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	c.Config.Name = "renamed-collection"
	c.Config.Ignore = []string{"vendor"}
	root, err := MarshalYAMLCollection(c)
	if err != nil {
		t.Fatalf("marshal collection: %v", err)
	}
	if !strings.Contains(string(root), "name: renamed-collection") {
		t.Fatalf("collection name not written:\n%s", root)
	}
	if !strings.Contains(string(root), "vendor") {
		t.Fatalf("ignore list not written:\n%s", root)
	}
	if !strings.Contains(string(root), "type: akamai-edgegrid") {
		t.Fatalf("collection auth not preserved:\n%s", root)
	}
}

func TestMarshalYAMLEnvironment(t *testing.T) {
	path := yamlFixture("edgegrid", "environments", "Local.yml")
	env, err := ParseYAMLEnvironment(path, "Local", readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	env.Vars = append(env.Vars, Variable{Name: "retries", Value: "3", Enabled: true, Kind: VarNumber})
	env.SecretNames = append(env.SecretNames, "apiKey")
	env.Vars = append(env.Vars, Variable{Name: "apiKey", Enabled: true, Secret: true})
	env.Extends = []string{"Base", "Shared"}
	env.Color = "#123456"

	out, err := MarshalYAMLEnvironment(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	again, err := ParseYAMLEnvironment(path, "Local", out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	if again.Name != "Local" {
		t.Fatalf("name = %q", again.Name)
	}
	if len(again.Extends) != 2 || again.Extends[0] != "Base" {
		t.Fatalf("extends = %v", again.Extends)
	}
	if again.Color != "#123456" {
		t.Fatalf("color = %q", again.Color)
	}
	var typed, secret *Variable
	for i := range again.Vars {
		switch again.Vars[i].Name {
		case "retries":
			typed = &again.Vars[i]
		case "apiKey":
			secret = &again.Vars[i]
		}
	}
	if typed == nil || typed.Kind != VarNumber || typed.Value != "3" {
		t.Fatalf("typed var = %+v", typed)
	}
	if secret == nil || !secret.Secret || secret.Value != "" {
		t.Fatalf("secret var = %+v", secret)
	}
	if len(again.SecretNames) != 1 || again.SecretNames[0] != "apiKey" {
		t.Fatalf("secrets = %v", again.SecretNames)
	}
}

func TestMarshalYAMLOutputIsCanonical(t *testing.T) {
	path := yamlFixture("descriptions", OpenCollectionFile)
	cfg, defaults, err := ParseYAMLCollection(path, readFixture(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c := &Collection{Config: cfg, Defaults: defaults, Source: defaults.Source}
	out, err := MarshalYAMLCollection(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(out)
	if !strings.HasPrefix(text, "opencollection:") {
		t.Fatalf("version must stay first:\n%s", text)
	}
	if strings.Contains(text, "\t") {
		t.Fatalf("tabs in output:\n%s", text)
	}
	if !strings.Contains(text, "\ninfo:\n  name:") {
		t.Fatalf("expected a two-space indent under info:\n%s", text)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Fatal("output must end with a newline")
	}
}

func TestMarshalYAMLKeepsItemLevelUnknownKeys(t *testing.T) {
	src := []byte(`info:
  name: extras
  type: http
http:
  method: GET
  url: https://example.com
  headers:
    - name: Accept
      value: application/json
      x-vendor: keep-me
  auth:
    type: oauth2
    grant_type: authorization_code
    additional_params:
      auth_req:
        headers:
          - name: X-Extra
            value: '1'
runtime:
  actions:
    - type: set-variable
      selector: { expression: res.body.id, method: jsonq }
      variable: { name: id, scope: runtime }
    - type: future-action
      payload: keep-me
`)
	req, err := ParseYAMLRequest("extras.yml", "extras.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out, err := MarshalYAMLRequest(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"x-vendor: keep-me", "additional_params:", "X-Extra", "future-action", "payload: keep-me"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("output lost %q:\n%s", want, out)
		}
	}
}

func TestMarshalYAMLFolderKeepsFolderType(t *testing.T) {
	src := []byte("info:\n  name: Users\n  type: folder\n  seq: 2\n")
	folder, err := ParseYAMLFolder("folder.yml", "users", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := MarshalYAMLFolder(folder)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "type: folder") {
		t.Fatalf("folder type lost:\n%s", out)
	}
	if strings.Contains(string(out), "type: http") {
		t.Fatalf("folder written as a request:\n%s", out)
	}
}

func TestMarshalYAMLEnvironmentKeepsUnknownKeys(t *testing.T) {
	src := []byte(`name: Local
x-vendor: keep
variables:
  - name: host
    value: https://a
    x-note: keep
`)
	env, err := ParseYAMLEnvironment("Local.yml", "Local", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	env.Vars[0].Value = "https://b"
	out, err := MarshalYAMLEnvironment(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"x-vendor: keep", "x-note: keep", "value: https://b"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("environment output lost %q:\n%s", want, out)
		}
	}
}

func TestMarshalYAMLKeepsDocsExtras(t *testing.T) {
	src := []byte(`info:
  name: docs
  type: http
http:
  url: https://example.com
docs:
  type: text/markdown
  x-vendor: keep
`)
	req, err := ParseYAMLRequest("docs.yml", "docs.yml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := MarshalYAMLRequest(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "x-vendor: keep") {
		t.Fatalf("docs extras lost:\n%s", out)
	}
	if !strings.Contains(string(out), "type: text/markdown") {
		t.Fatalf("docs type lost:\n%s", out)
	}
}
