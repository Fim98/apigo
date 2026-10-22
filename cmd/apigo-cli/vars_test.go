package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// varsCollection writes a small collection with two folder levels, an
// environment and a .env file.
func varsCollection(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"bruno.json": `{"version":"1","name":"vars-demo","type":"collection"}`,
		"collection.bru": `meta {
  name: vars-demo
}

vars:pre-request {
  shared: collection
}
`,
		"users/folder.bru": `meta {
  name: Users
}

vars:pre-request {
  shared: users
  tenant: users
}
`,
		"users/list.bru": `meta {
  name: List
  type: http
  seq: 1
}

get {
  url: https://example.com/users
}

vars:pre-request {
  shared: request
}
`,
		"environments/Local.bru": `vars {
  region: local
  token: plain-token
}
`,
		"environments/Prod.bru": `vars:secret [
  apiKey
]
`,
		".env": "LOCAL_TOKEN=secret-value\nREGION=from-dotenv\n",
	}
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

func TestVarsCommandLayers(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--env", "Local", "--request", "users/list.bru", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	text := out.String()

	for _, want := range []string{
		"vars-demo",
		"request: users/list.bru",
		"environment: Local",
		"collection (1)",
		"  shared = collection (shadowed by request)",
		"folder (2)",
		"  tenant = users",
		"  shared = users (shadowed by request)",
		"request (1)",
		"  shared = request",
		"region = local",
		"process.env (2 from .env)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
	// Values of secrets and of .env never reach the output.
	if strings.Contains(text, "secret-value") {
		t.Fatalf(".env value leaked:\n%s", text)
	}
	if strings.Contains(text, "from-dotenv") {
		t.Fatalf(".env value leaked:\n%s", text)
	}
}

func TestVarsCommandMasksEnvironmentSecrets(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--env", "Prod", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "apiKey = ********") {
		t.Fatalf("secret not masked:\n%s", out.String())
	}
}

func TestVarsCommandSetOverrides(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--env", "Local", "--set", "region=override", "--set", "added=1", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	for _, want := range []string{"region = override", "added = 1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestVarsCommandJSON(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--json", "--env", "Local", "--request", "users/list.bru", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}

	var summary struct {
		Root        string   `json:"root"`
		Request     string   `json:"request"`
		Environment string   `json:"environment"`
		Precedence  []string `json:"precedence"`
		DotEnv      []string `json:"dotenv"`
		Variables   []struct {
			Name     string `json:"name"`
			Value    string `json:"value"`
			Scope    string `json:"scope"`
			Kind     string `json:"kind"`
			Secret   bool   `json:"secret"`
			Shadowed bool   `json:"shadowed"`
		} `json:"variables"`
	}
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}

	if summary.Request != "users/list.bru" || summary.Environment != "Local" {
		t.Fatalf("header = %+v", summary)
	}
	if got := strings.Join(summary.Precedence, ","); got != "global,collection,environment,folder,request,oauth2,runtime,prompt" {
		t.Fatalf("precedence = %s", got)
	}
	if got := strings.Join(summary.DotEnv, ","); got != "LOCAL_TOKEN,REGION" {
		t.Fatalf("dotenv = %s", got)
	}

	type key struct{ name, scope string }
	seen := map[key]string{}
	for _, v := range summary.Variables {
		seen[key{v.Name, v.Scope}] = v.Value
		if v.Secret && v.Value != maskedValue {
			t.Fatalf("secret %s not masked: %q", v.Name, v.Value)
		}
	}
	if seen[key{"shared", "request"}] != "request" {
		t.Fatalf("request layer = %v", seen)
	}
	if seen[key{"shared", "folder"}] != "users" {
		t.Fatalf("folder layer = %v", seen)
	}
	if seen[key{"shared", "collection"}] != "collection" {
		t.Fatalf("collection layer = %v", seen)
	}
	if seen[key{"region", "environment"}] != "local" {
		t.Fatalf("environment layer = %v", seen)
	}
	// The process.env namespace itself is not in the variables list; only .env's
	// names are reported.
	for k := range seen {
		if strings.HasPrefix(k.name, "process.env.") {
			t.Fatalf("process.env entry in the output: %+v", k)
		}
	}
}

func TestVarsCommandErrors(t *testing.T) {
	dir := varsCollection(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no path", nil, "missing collection directory"},
		{"unknown flag", []string{"--nope", dir}, "unknown flag"},
		{"missing flag value", []string{"--env"}, "needs a value"},
		{"two collections", []string{dir, dir}, "one collection at a time"},
		{"unknown environment", []string{"--env", "Nope", dir}, `no environment "Nope"`},
		{"unknown request", []string{"--request", "nope.bru", dir}, "no request or folder"},
		{"bad set", []string{"--set", "novalue", dir}, "NAME=VALUE"},
		{"missing collection", []string{filepath.Join(dir, "nope")}, "collection not found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := runVars(tc.args, &out, &errOut); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}
			if !strings.Contains(errOut.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tc.want)
			}
		})
	}
}

func TestVarsCommandAcceptsWindowsStylePath(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--request", `users\list.bru`, dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "request: users/list.bru") {
		t.Fatalf("output missing the request:\n%s", out.String())
	}
}

func TestVarsCommandRootDefaults(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if strings.Contains(out.String(), "request:") {
		t.Fatalf("root output should not name a request:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "collection (1)") {
		t.Fatalf("collection layer missing:\n%s", out.String())
	}
}

// TestHelpMentionsVars keeps the command discoverable from `apigo help`.
func TestHelpMentionsVars(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"help"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "vars") {
		t.Fatalf("help missing vars:\n%s", out.String())
	}
}

func TestVarsCommandText(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	code := runVars([]string{"--env", "Local", "--request", "users/list.bru", "--text", "https://{{region}}/{{tenant}}", dir}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	// Both the environment and the folder layer take part.
	if got := strings.TrimSpace(out.String()); got != "https://local/users" {
		t.Fatalf("text = %q", got)
	}
	if strings.Contains(errOut.String(), "unresolved") {
		t.Fatalf("unexpected unresolved line: %q", errOut.String())
	}

	// A missing name is reported, and the command still succeeds.
	out.Reset()
	errOut.Reset()
	code = runVars([]string{"--env", "Local", "--request", "users/list.bru", "--text", "{{region}}/{{nope}}", dir}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "local/{{nope}}" {
		t.Fatalf("stdout = %q, want the result alone so it can be piped", got)
	}
	if !strings.Contains(errOut.String(), "unresolved: nope") {
		t.Fatalf("stderr = %q, want the unresolved names", errOut.String())
	}
}

// TestVarsCommandTextMasksSecrets keeps a secret from reaching the terminal or a
// piped command.
func TestVarsCommandTextMasksSecrets(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--env", "Prod", "--text", "Key={{apiKey}}", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "Key="+maskedValue {
		t.Fatalf("text = %q", got)
	}

	// The JSON form carries the flag too.
	out.Reset()
	errOut.Reset()
	if code := runVars([]string{"--json", "--env", "Prod", "--text", "{{apiKey}}", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var payload struct {
		Text   string `json:"text"`
		Secret bool   `json:"secret"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if payload.Text != maskedValue || !payload.Secret {
		t.Fatalf("payload = %+v", payload)
	}
}

// TestVarsCommandEmptyText checks that --text "" still selects interpolation
// mode instead of falling back to the layer listing.
func TestVarsCommandEmptyText(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	if code := runVars([]string{"--text", "", dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "" {
		t.Fatalf("stdout = %q, want an empty result", got)
	}
	if strings.Contains(out.String(), "collection (") {
		t.Fatalf("--text fell back to the layer listing:\n%s", out.String())
	}
}

func TestVarsCommandTextJSON(t *testing.T) {
	dir := varsCollection(t)

	var out, errOut bytes.Buffer
	code := runVars([]string{"--json", "--env", "Local", "--request", "users/list.bru", "--text", "{{shared}}-{{nope}}", dir}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}

	var payload struct {
		Text       string   `json:"text"`
		Kind       string   `json:"kind"`
		Unresolved []string `json:"unresolved"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if payload.Text != "request-{{nope}}" || payload.Kind != "string" {
		t.Fatalf("payload = %+v", payload)
	}
	if strings.Join(payload.Unresolved, ",") != "nope" {
		t.Fatalf("unresolved = %v", payload.Unresolved)
	}
}
