package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unformatted is a request whose blocks are out of canonical order and whose
// indentation is wrong.
const unformatted = `docs {
  Notes.
}

get {
    url: https://example.com
}

meta {
  name: messy
}
`

const formatted = `meta {
  name: messy
}

get {
  url: https://example.com
}

docs {
  Notes.
}
`

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFmtWritesCanonicalFile(t *testing.T) {
	path := writeTemp(t, "messy.bru", unformatted)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{path}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != formatted {
		t.Fatalf("file = %q, want %q", got, formatted)
	}
}

func TestFmtCheckReportsWithoutWriting(t *testing.T) {
	path := writeTemp(t, "messy.bru", unformatted)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{"--check", path}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1 (changes needed); stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "messy.bru") {
		t.Fatalf("stdout %q", out.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != unformatted {
		t.Fatalf("--check must not write, file = %q", after)
	}

	// Once canonical, --check succeeds.
	var out2, errOut2 bytes.Buffer
	if code := runFmt([]string{path}, &out2, &errOut2); code != 0 {
		t.Fatalf("format: exit %d, stderr %q", code, errOut2.String())
	}
	var out3, errOut3 bytes.Buffer
	if code := runFmt([]string{"--check", path}, &out3, &errOut3); code != 0 {
		t.Fatalf("check after format: exit %d, stdout %q", code, out3.String())
	}
}

func TestFmtStdout(t *testing.T) {
	path := writeTemp(t, "messy.bru", unformatted)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{"--stdout", path}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if out.String() != formatted {
		t.Fatalf("stdout = %q", out.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != unformatted {
		t.Fatal("--stdout must not write")
	}
}

func TestFmtEnvironmentFile(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(envDir, "Local.bru")
	if err := os.WriteFile(path, []byte("vars {\n    baseUrl: https://x\n}\ncolor: #fff\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := runFmt([]string{path}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "vars {\n  baseUrl: https://x\n}\ncolor: #fff\n"
	if string(got) != want {
		t.Fatalf("env file = %q, want %q", got, want)
	}
}

func TestFmtDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bru"), []byte(unformatted), 0o644); err != nil {
		t.Fatal(err)
	}
	hidden := filepath.Join(dir, ".git")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "b.bru"), []byte(unformatted), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := runFmt([]string{dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if strings.Contains(out.String(), ".git") {
		t.Fatalf("hidden directories must be skipped: %q", out.String())
	}
	hiddenFile, err := os.ReadFile(filepath.Join(hidden, "b.bru"))
	if err != nil {
		t.Fatal(err)
	}
	if string(hiddenFile) != unformatted {
		t.Fatal("hidden directory was modified")
	}
}

func TestFmtErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runFmt(nil, &out, &errOut); code != 2 {
		t.Fatalf("missing path: exit %d, want 2", code)
	}

	if code := runFmt([]string{"--nope", "x"}, &out, &errOut); code != 2 {
		t.Fatalf("unknown flag: exit %d, want 2", code)
	}

	if code := runFmt([]string{filepath.Join(t.TempDir(), "missing.bru")}, &out, &errOut); code != 2 {
		t.Fatalf("missing file: exit %d, want 2", code)
	}

	broken := writeTemp(t, "broken.bru", "meta {\n  name: x\n")
	if code := runFmt([]string{broken}, &out, &errOut); code != 2 {
		t.Fatalf("broken file: exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unterminated meta block") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestInspectTree(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "collection", "testdata", "sample")

	var out, errOut bytes.Buffer
	if code := runInspect([]string{root}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	got := out.String()
	for _, want := range []string{"sample", "Users/", "Admin/", "GET    List users", "DELETE Delete user", "3 request(s), 2 environment(s)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestInspectJSON(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "collection", "testdata", "sample")

	var out, errOut bytes.Buffer
	if code := runInspect([]string{"--json", root}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	for _, want := range []string{`"name": "sample"`, `"method": "GET"`, `"environments"`, `"Local"`, `"extends": [`, `"Base"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("json missing %q:\n%s", want, out.String())
		}
	}
}

func TestInspectErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runInspect(nil, &out, &errOut); code != 2 {
		t.Fatalf("missing path: exit %d, want 2", code)
	}
	if code := runInspect([]string{"--json", "a", "b"}, &out, &errOut); code != 2 {
		t.Fatalf("two collections: exit %d, want 2", code)
	}
	if code := runInspect([]string{filepath.Join(t.TempDir(), "nope")}, &out, &errOut); code != 2 {
		t.Fatalf("missing collection: exit %d, want 2", code)
	}
}

func TestHelpMentionsCommands(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"help"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"fmt", "inspect"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

// TestFmtYAMLCollection formats an OpenCollection tree: the root file, a folder
// and an item, plus an environment.
func TestFmtYAMLCollection(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"opencollection.yml":     "info:\n    name: yaml-collection\n",
		"users/folder.yml":       "info:\n    name: Users\n    type: folder\n",
		"users/get.yml":          "info:\n    name: Get\n    type: http\nhttp:\n    method: GET\n    url: https://example.com\n",
		"environments/Local.yml": "name: Local\nvariables:\n    - name: host\n      value: https://a\n",
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

	var out, errOut bytes.Buffer
	if code := runFmt([]string{"--check", dir}, &out, &errOut); code != 1 {
		t.Fatalf("check before format: exit %d, want 1; stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "opencollection.yml") || !strings.Contains(out.String(), "Local.yml") {
		t.Fatalf("check output = %q", out.String())
	}

	out.Reset()
	errOut.Reset()
	if code := runFmt([]string{dir}, &out, &errOut); code != 0 {
		t.Fatalf("format: exit %d, stderr %q", code, errOut.String())
	}

	item, err := os.ReadFile(filepath.Join(dir, "users", "get.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(item), "\n  method: GET\n") {
		t.Fatalf("item not canonicalised:\n%s", item)
	}
	root, err := os.ReadFile(filepath.Join(dir, "opencollection.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(root), `opencollection: "1.0.0"`) {
		t.Fatalf("root file missing the format version:\n%s", root)
	}

	out.Reset()
	errOut.Reset()
	if code := runFmt([]string{"--check", dir}, &out, &errOut); code != 0 {
		t.Fatalf("check after format: exit %d\n%s", code, out.String())
	}
}

// TestFmtLeavesUnrelatedYAMLAlone guards against rewriting files that only look
// like YAML: a compose file or CI config must never be touched.
func TestFmtLeavesUnrelatedYAMLAlone(t *testing.T) {
	dir := t.TempDir()
	compose := "services:\n    web:\n        image: nginx\n"
	path := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(path, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "request.bru"), []byte(unformatted), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := runFmt([]string{dir}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != compose {
		t.Fatalf("compose file was rewritten:\n%s", after)
	}
	if strings.Contains(out.String(), "docker-compose.yml") {
		t.Fatalf("compose file reported: %q", out.String())
	}

	// An explicit path is an error, not a silent rewrite.
	out.Reset()
	errOut.Reset()
	if code := runFmt([]string{path}, &out, &errOut); code != 2 {
		t.Fatalf("explicit compose file: exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "not an OpenCollection file") {
		t.Fatalf("stderr %q", errOut.String())
	}
}
