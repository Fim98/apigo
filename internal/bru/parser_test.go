package bru

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func parseFixture(t *testing.T, name string) *File {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", name)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	f, err := Parse(path, src)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return f
}

func TestParseRequestMeta(t *testing.T) {
	f := parseFixture(t, "request.bru")

	meta, ok := f.FirstDict("meta")
	if !ok {
		t.Fatal("meta block missing")
	}
	if name, _ := meta.GetString("name"); name != "Create user" {
		t.Fatalf("meta.name = %q", name)
	}
	if seq, _ := meta.GetString("seq"); seq != "3" {
		t.Fatalf("meta.seq = %q", seq)
	}
	tags, ok := meta.Get("tags")
	if !ok || tags.Kind != ValueList {
		t.Fatalf("meta.tags = %+v", tags)
	}
	if want := []string{"smoke", "users"}; strings.Join(tags.Items, ",") != strings.Join(want, ",") {
		t.Fatalf("tags = %v, want %v", tags.Items, want)
	}
}

func TestParseRequestMethodBlock(t *testing.T) {
	f := parseFixture(t, "request.bru")

	if blocks := f.FindBlocks("post"); len(blocks) != 1 {
		t.Fatalf("post blocks = %d", len(blocks))
	}
	post, ok := f.FirstDict("post")
	if !ok {
		t.Fatal("post block missing")
	}
	if url, _ := post.GetString("url"); url != "{{baseUrl}}/users/:id" {
		t.Fatalf("url = %q", url)
	}
	if body, _ := post.GetString("body"); body != "json" {
		t.Fatalf("body = %q", body)
	}
}

func TestParseDisabledPairs(t *testing.T) {
	f := parseFixture(t, "request.bru")

	params, ok := f.FirstDict("params:query")
	if !ok {
		t.Fatal("params:query missing")
	}
	if len(params.Pairs) != 2 {
		t.Fatalf("pairs = %d", len(params.Pairs))
	}
	if !params.Pairs[1].Disabled || params.Pairs[1].Key != "debug" {
		t.Fatalf("second pair = %+v", params.Pairs[1])
	}
	if params.Pairs[0].Disabled {
		t.Fatal("first pair must be enabled")
	}
}

func TestParseAnnotations(t *testing.T) {
	f := parseFixture(t, "request.bru")

	vars, ok := f.FirstDict("vars:pre-request")
	if !ok {
		t.Fatal("vars:pre-request missing")
	}
	if len(vars.Pairs) != 3 {
		t.Fatalf("pairs = %d", len(vars.Pairs))
	}
	if got := vars.Pairs[1].Annotations; len(got) != 1 || got[0].Name != "number" || got[0].HasArg {
		t.Fatalf("annotations = %+v", got)
	}
	if got := vars.Pairs[2].Annotations; len(got) != 1 || got[0].Name != "boolean" {
		t.Fatalf("annotations = %+v", got)
	}
}

func headerAnnotations(t *testing.T, d *DictBlock) []string {
	t.Helper()
	var names []string
	for _, p := range d.Pairs {
		for _, a := range p.Annotations {
			names = append(names, a.Name)
		}
	}
	return names
}

func TestParseTextBlocks(t *testing.T) {
	f := parseFixture(t, "request.bru")

	body, ok := f.FirstText("body:json")
	if !ok {
		t.Fatal("body:json missing")
	}
	want := "  {\n    \"name\": \"Ada\",\n    \"tags\": [\"math\"]\n  }"
	if body.Text != want {
		t.Fatalf("body text = %q, want %q", body.Text, want)
	}

	script, ok := f.FirstText("script:post-response")
	if !ok {
		t.Fatal("script:post-response missing")
	}
	if !strings.Contains(script.Text, "res.getStatus()") {
		t.Fatalf("script text = %q", script.Text)
	}
}

func TestParseAssertKeys(t *testing.T) {
	f := parseFixture(t, "request.bru")

	assert, ok := f.FirstDict("assert")
	if !ok {
		t.Fatal("assert missing")
	}
	if !assert.AssertSyntax {
		t.Fatal("assert block must be marked as assert syntax")
	}
	if len(assert.Pairs) != 2 {
		t.Fatalf("pairs = %d", len(assert.Pairs))
	}
	if assert.Pairs[0].Key != "res.status" || assert.Pairs[0].Value.Text != "eq 200" {
		t.Fatalf("pair = %+v", assert.Pairs[0])
	}
}

func TestParseCollectionAndFolder(t *testing.T) {
	folder := parseFixture(t, "folder.bru")
	meta, _ := folder.FirstDict("meta")
	if name, _ := meta.GetString("name"); name != "Users" {
		t.Fatalf("folder name = %q", name)
	}

	collection := parseFixture(t, "collection.bru")
	meta, _ = collection.FirstDict("meta")
	if name, _ := meta.GetString("type"); name != "collection" {
		t.Fatalf("collection type = %q", name)
	}
	headers, _ := collection.FirstDict("headers")
	if got := headerAnnotations(t, headers); len(got) != 1 || got[0] != "description" {
		t.Fatalf("header annotations = %v", got)
	}
	if v, _ := headers.GetString("X-Client"); v != "apigo" {
		t.Fatalf("X-Client = %q", v)
	}
}

func TestParseUnknownBlocksPreserved(t *testing.T) {
	f := parseFixture(t, "unknown.bru")

	raw, ok := f.Blocks[2].(*RawBlock)
	if !ok {
		t.Fatalf("third block is %T, want *RawBlock", f.Blocks[2])
	}
	if raw.Name != "future:block" {
		t.Fatalf("raw name = %q", raw.Name)
	}
	for _, want := range []string{"keep: this", "nested: {", "deep: true"} {
		if !strings.Contains(raw.Raw, want) {
			t.Fatalf("raw block lost %q:\n%s", want, raw.Raw)
		}
	}
	if !strings.Contains(raw.Raw, "}") {
		t.Fatalf("raw block lost its closing brace:\n%s", raw.Raw)
	}

	last, ok := f.Blocks[len(f.Blocks)-1].(*RawBlock)
	if !ok || last.Name != "another:extension" {
		t.Fatalf("last block = %+v", f.Blocks[len(f.Blocks)-1])
	}
	if !strings.Contains(last.Raw, "two") {
		t.Fatalf("list raw block = %q", last.Raw)
	}
}

func TestParseQuotedKeysAndMultipart(t *testing.T) {
	f := parseFixture(t, "multiline.bru")

	headers, ok := f.FirstDict("headers")
	if !ok {
		t.Fatal("headers missing")
	}
	if headers.Pairs[0].Key != "X-Weird: Key" {
		t.Fatalf("quoted key = %q", headers.Pairs[0].Key)
	}
	if headers.Pairs[0].Value.Text != "value with: colon" {
		t.Fatalf("quoted value = %q", headers.Pairs[0].Value.Text)
	}

	form, ok := f.FirstDict("body:multipart-form")
	if !ok {
		t.Fatal("multipart block missing")
	}
	if form.Pairs[2].Value.Text != "@file(/tmp/a.txt) @contentType(text/plain)" {
		t.Fatalf("multipart value = %q", form.Pairs[2].Value.Text)
	}
	if !form.Pairs[1].Disabled {
		t.Fatal("disabled multipart field lost its marker")
	}
}

func TestParseEnv(t *testing.T) {
	path := filepath.Join("testdata", "fixtures", "environment.bru")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	env, err := ParseEnv(path, src)
	if err != nil {
		t.Fatalf("parse env: %v", err)
	}
	if len(env.Variables) != 3 {
		t.Fatalf("variables = %d", len(env.Variables))
	}
	if env.Variables[0].Key != "baseUrl" || env.Variables[0].Value.Text != "https://staging.example.com" {
		t.Fatalf("variable 0 = %+v", env.Variables[0])
	}
	if env.Variables[1].Value.Kind != ValueMultiline || env.Variables[1].Value.Text != "multi\nline" {
		t.Fatalf("variable 1 = %+v", env.Variables[1])
	}
	if len(env.Variables[2].Annotations) != 1 || env.Variables[2].Annotations[0].Name != "number" {
		t.Fatalf("variable 2 annotations = %+v", env.Variables[2].Annotations)
	}
	if env.Variables[2].Key != "retries" || env.Variables[2].Value.Text != "3" {
		t.Fatalf("variable 2 = %+v", env.Variables[2])
	}
	if want := "apiKey,clientSecret"; strings.Join(env.Secrets, ",") != want {
		t.Fatalf("secrets = %v", env.Secrets)
	}
	if len(env.Extends) != 1 || env.Extends[0] != "Base" {
		t.Fatalf("extends = %v", env.Extends)
	}
	if env.Color != "#4caf50" {
		t.Fatalf("color = %q", env.Color)
	}
}

func TestParseErrorCarriesPosition(t *testing.T) {
	src := []byte("meta {\n  name: broken\n")
	f, err := Parse("broken.bru", src)
	if err == nil {
		t.Fatal("expected an error")
	}
	if f == nil {
		t.Fatal("a partial AST must be returned with the error")
	}
	var list ErrorList
	if !errors.As(err, &list) || len(list) == 0 {
		t.Fatalf("error type %T", err)
	}
	if list[0].Pos.Line != 1 || list[0].Path != "broken.bru" {
		t.Fatalf("error = %+v", list[0])
	}
}

func TestParseBOMAndCRLF(t *testing.T) {
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("meta {\r\n  name: crlf\r\n}\r\n\r\nget {\r\n  url: https://example.com\r\n}\r\n")...)
	f, err := Parse("crlf.bru", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	meta, ok := f.FirstDict("meta")
	if !ok {
		t.Fatal("meta missing")
	}
	if name, _ := meta.GetString("name"); name != "crlf" {
		t.Fatalf("name = %q", name)
	}
}

func TestParseDuplicateKeysPreserved(t *testing.T) {
	src := []byte("params:query {\n  tag: a\n  tag: b\n}\n")
	f, err := Parse("dup.bru", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	params, _ := f.FirstDict("params:query")
	if len(params.Pairs) != 2 || params.Pairs[0].Value.Text != "a" || params.Pairs[1].Value.Text != "b" {
		t.Fatalf("pairs = %+v", params.Pairs)
	}
}

func TestParseEmptyFile(t *testing.T) {
	f, err := Parse("empty.bru", nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(f.Blocks) != 0 {
		t.Fatalf("blocks = %d", len(f.Blocks))
	}
	if got := string(Format(f)); got != "" {
		t.Fatalf("format = %q", got)
	}
}

func TestFormatCanonicalOrder(t *testing.T) {
	src := []byte(`settings {
  timeout: 1000
}

docs {
  Notes.
}

get {
  url: https://example.com
}

headers {
  Accept: */*
}

meta {
  name: order
}
`)
	f, err := Parse("order.bru", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := FormatString(f)
	want := []string{"meta {", "get {", "headers {", "settings {", "docs {"}
	last := -1
	for _, name := range want {
		idx := strings.Index(got, name)
		if idx < 0 {
			t.Fatalf("missing %q in\n%s", name, got)
		}
		if idx < last {
			t.Fatalf("%q out of canonical order in\n%s", name, got)
		}
		last = idx
	}
}

func TestFormatMatchesGolden(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDEN") == "1"
	fixtures, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.bru"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range fixtures {
		name := strings.TrimSuffix(filepath.Base(path), ".bru")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		var got string
		if name == "environment" {
			env, err := ParseEnv(path, src)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			got = string(FormatEnv(env))
		} else {
			f, err := Parse(path, src)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			got = FormatString(f)
		}

		goldenPath := filepath.Join("testdata", "fixtures", name+".golden")
		if update {
			if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("%s: read golden: %v (run UPDATE_GOLDEN=1 go test ./internal/bru)", name, err)
		}
		if got != string(want) {
			t.Fatalf("%s: canonical output differs\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
		}
	}
}

// digest renders an AST without spans so two parses of equivalent input compare
// equal.
func digest(f *File) string {
	blocks := make([]Block, len(f.Blocks))
	copy(blocks, f.Blocks)
	sort.SliceStable(blocks, func(i, j int) bool {
		ri, rj := blockRank(blocks[i]), blockRank(blocks[j])
		if ri != rj {
			return ri < rj
		}
		return secondaryRank(blocks[i]) < secondaryRank(blocks[j])
	})

	var b strings.Builder
	for _, blk := range blocks {
		switch v := blk.(type) {
		case *DictBlock:
			fmt.Fprintf(&b, "dict %q assert=%v\n", v.Name, v.AssertSyntax)
			for _, p := range canonicalPairs(v.Pairs) {
				fmt.Fprintf(&b, "  pair %q disabled=%v\n", p.Key, p.Disabled)
				for _, a := range p.Annotations {
					fmt.Fprintf(&b, "    ann %q arg=%q has=%v\n", a.Name, a.Arg, a.HasArg)
				}
				fmt.Fprintf(&b, "    kind %d text %q\n", p.Value.Kind, p.Value.Text)
				for _, item := range p.Value.Items {
					fmt.Fprintf(&b, "    item %q\n", item)
				}
			}
		case *TextBlock:
			fmt.Fprintf(&b, "text %q example=%v\n%q\n", v.Name, v.Example, v.Text)
		case *RawBlock:
			fmt.Fprintf(&b, "raw %q\n%q\n", v.Name, v.Raw)
		default:
			fmt.Fprintf(&b, "unknown block %T\n", blk)
		}
	}
	return b.String()
}

func TestSemanticRoundTrip(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.bru"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range fixtures {
		name := filepath.Base(path)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(name, "environment") {
			first, err := ParseEnv(path, src)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			printed := FormatEnv(first)
			second, err := ParseEnv(path, printed)
			if err != nil {
				t.Fatalf("%s: reparse: %v", name, err)
			}
			if !envEqual(first, second) {
				t.Fatalf("%s: environment model changed across a round trip", name)
			}
			if string(FormatEnv(second)) != string(printed) {
				t.Fatalf("%s: formatting is not idempotent", name)
			}
			continue
		}

		first, err := Parse(path, src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		printed := Format(first)
		second, err := Parse(path, printed)
		if err != nil {
			t.Fatalf("%s: reparse: %v\n%s", name, err, printed)
		}
		if digest(first) != digest(second) {
			t.Fatalf("%s: AST changed across a round trip\n--- first ---\n%s\n--- second ---\n%s", name, digest(first), digest(second))
		}
		if again := Format(second); string(again) != string(printed) {
			t.Fatalf("%s: formatting is not idempotent\n--- first ---\n%s\n--- second ---\n%s", name, printed, again)
		}
	}
}

// canonicalPairs puts enabled pairs before disabled ones, the order the printer
// emits, so a digest survives canonicalisation.
func canonicalPairs(pairs []Pair) []Pair {
	out := make([]Pair, 0, len(pairs))
	for _, p := range pairs {
		if !p.Disabled {
			out = append(out, p)
		}
	}
	for _, p := range pairs {
		if p.Disabled {
			out = append(out, p)
		}
	}
	return out
}

func envEqual(a, b *EnvFile) bool {
	if a.Color != b.Color || strings.Join(a.Extends, "\x00") != strings.Join(b.Extends, "\x00") || strings.Join(a.Secrets, "\x00") != strings.Join(b.Secrets, "\x00") {
		return false
	}
	if len(a.Variables) != len(b.Variables) {
		return false
	}
	av, bv := canonicalPairs(a.Variables), canonicalPairs(b.Variables)
	for i := range av {
		if av[i].Key != bv[i].Key || av[i].Value.String() != bv[i].Value.String() || av[i].Disabled != bv[i].Disabled {
			return false
		}
		if len(av[i].Annotations) != len(bv[i].Annotations) {
			return false
		}
		for j := range av[i].Annotations {
			if av[i].Annotations[j].Name != bv[i].Annotations[j].Name || av[i].Annotations[j].Arg != bv[i].Annotations[j].Arg {
				return false
			}
		}
	}
	return true
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"meta {\n  name: a\n}\n",
		"get {\n  url: https://x/{{v}}\n}\n",
		"body:json {\n  {\"a\": 1}\n}\n",
		"tags: [\n  a\n]\n",
		"@description('x')\nkey: value\n",
		"'''\n  x\n'''\n",
		"vars:secret [\n  a\n]\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		file, err := Parse("fuzz.bru", []byte(src))
		if file == nil {
			t.Fatal("Parse returned a nil file")
		}
		if err != nil {
			// A partial AST is allowed; printing it must not panic.
			_ = Format(file)
			return
		}
		printed := Format(file)
		second, err := Parse("fuzz.bru", printed)
		if err != nil {
			t.Fatalf("reparse of formatted output failed: %v\ninput %q\nformatted %q", err, src, printed)
		}
		if digest(file) != digest(second) {
			t.Fatalf("round trip changed the AST\ninput %q\nformatted %q", src, printed)
		}
		if again := Format(second); string(again) != string(printed) {
			t.Fatalf("formatting is not idempotent\ninput %q\nfirst %q\nsecond %q", src, printed, again)
		}
	})
}
