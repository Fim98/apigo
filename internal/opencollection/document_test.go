package opencollection

import (
	"strings"
	"testing"
)

const sample = `opencollection: '1.0.0'

info:
  name: demo
  type: http

http:
  method: GET
  url: https://example.com
  headers:
    - name: Accept
      value: application/json
    - name: X-Debug
      value: '1'
      disabled: true

x-vendor-field:
  nested: true
`

func TestParsePreservesOrderAndUnknownKeys(t *testing.T) {
	doc, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	root := doc.Root()
	if !root.IsMap() {
		t.Fatalf("root kind = %v", root.Kind())
	}
	if got := root.Get("info").Get("name").String(); got != "demo" {
		t.Fatalf("info.name = %q", got)
	}

	var keys []string
	root.Each(func(key string, _ *Node) bool {
		keys = append(keys, key)
		return true
	})
	want := "opencollection,info,http,x-vendor-field"
	if strings.Join(keys, ",") != want {
		t.Fatalf("keys = %v, want %s", keys, want)
	}

	if root.Get("x-vendor-field").Get("nested").String() != "true" {
		t.Fatal("unknown key not readable")
	}
}

func TestNodeScalarAccessors(t *testing.T) {
	doc, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	http := doc.Root().Get("http")

	if got := http.Get("method").String(); got != "GET" {
		t.Fatalf("method = %q", got)
	}
	if got, ok := http.Get("method").Bool(); ok || got {
		t.Fatal("a non-boolean scalar must not decode as a bool")
	}

	headers := http.Get("headers")
	if !headers.IsSequence() || len(headers.Items()) != 2 {
		t.Fatalf("headers = %+v", headers)
	}
	if disabled, ok := headers.Items()[1].Get("disabled").Bool(); !ok || !disabled {
		t.Fatalf("disabled = %v ok=%v", disabled, ok)
	}

	if line := http.Get("url").Line(); line != 9 {
		t.Fatalf("url line = %d, want 9", line)
	}
}

func TestNodeMissingValuesAreSafe(t *testing.T) {
	doc, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	missing := doc.Root().Get("nope").Get("deeper")
	if missing.Valid() {
		t.Fatal("missing node must be invalid")
	}
	if missing.String() != "" || missing.Len() != 0 || missing.Items() != nil {
		t.Fatal("missing node must behave like a zero value")
	}
	if missing.Get("anything").Valid() {
		t.Fatal("Get on a missing node must stay missing")
	}
}

func TestDocumentBytesKeepsUnknownFields(t *testing.T) {
	doc, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Bytes()
	if err != nil {
		t.Fatalf("bytes: %v", err)
	}
	text := string(out)
	for _, want := range []string{"x-vendor-field:", "nested: true", "opencollection:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lost %q:\n%s", want, text)
		}
	}
}

func TestParseEmptyDocument(t *testing.T) {
	for _, src := range []string{"", "\n\n", "# only a comment\n"} {
		doc, err := Parse([]byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if doc.Root().Valid() {
			t.Fatalf("%q: expected no root", src)
		}
		out, err := doc.Bytes()
		if err != nil || len(out) != 0 {
			t.Fatalf("%q: bytes = %q err=%v", src, out, err)
		}
	}
}

func TestParseErrorCarriesLine(t *testing.T) {
	_, err := Parse([]byte("info:\n  name: [unclosed\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	ocErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type %T", err)
	}
	if ocErr.Line == 0 {
		t.Fatalf("error has no line: %v", ocErr)
	}
	if !strings.Contains(ocErr.Error(), "opencollection:") {
		t.Fatalf("error text %q", ocErr.Error())
	}
}

func TestFragmentDocument(t *testing.T) {
	doc, err := Parse([]byte("a: 1\nb: [x, y]\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewDocument(doc.Root().Get("b")).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// The node keeps its source style, which is what makes fragments stable.
	if got := strings.TrimSpace(string(out)); got != "[x, y]" {
		t.Fatalf("fragment = %q", got)
	}
}

func TestSetReplacesInPlaceAndAppends(t *testing.T) {
	doc, err := Parse([]byte("a: 1\nb: 2\nc: 3\n"))
	if err != nil {
		t.Fatal(err)
	}
	root := doc.Root()
	if err := root.Set("b", "two"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := root.Set("d", true); err != nil {
		t.Fatalf("set: %v", err)
	}

	var keys []string
	root.Each(func(key string, _ *Node) bool {
		keys = append(keys, key)
		return true
	})
	if got := strings.Join(keys, ","); got != "a,b,c,d" {
		t.Fatalf("key order = %s, want a,b,c,d (existing keys keep their place)", got)
	}
	if root.Get("b").String() != "two" {
		t.Fatalf("b = %q", root.Get("b").String())
	}
	if v, ok := root.Get("d").Bool(); !ok || !v {
		t.Fatalf("d = %+v", root.Get("d"))
	}
}

func TestSetOnNullCreatesMapping(t *testing.T) {
	doc, err := Parse([]byte("info:\nhttp:\n  method: GET\n"))
	if err != nil {
		t.Fatal(err)
	}
	// `info:` with no value is a present-but-null node: Set turns it into a
	// mapping in place.
	info := doc.Root().Get("info")
	if !info.Valid() || !info.IsNull() {
		t.Fatalf("info = valid %v null %v", info.Valid(), info.IsNull())
	}
	if err := info.Set("name", "filled"); err != nil {
		t.Fatalf("set on null: %v", err)
	}
	if info.Get("name").String() != "filled" {
		t.Fatal("null node was not turned into a mapping")
	}

	http := doc.Root().Get("http")
	if err := http.Set("url", "https://example.com"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if http.Get("url").String() != "https://example.com" {
		t.Fatalf("url = %q", http.Get("url").String())
	}
	if http.Get("method").String() != "GET" {
		t.Fatal("existing entries must survive")
	}
}

func TestDeleteAndAppend(t *testing.T) {
	doc, err := Parse([]byte("tags: [a]\nlist:\n"))
	if err != nil {
		t.Fatal(err)
	}
	root := doc.Root()
	if !root.Delete("tags") {
		t.Fatal("delete reported false")
	}
	if root.Delete("tags") {
		t.Fatal("second delete must report false")
	}

	list := root.Get("list")
	item, _ := Value("x")
	list.Append(item)
	if got := strings.Join(list.Strings(), ","); got != "x" {
		t.Fatalf("list = %q", got)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	doc, err := Parse([]byte("info:\n  name: original\n"))
	if err != nil {
		t.Fatal(err)
	}
	clone := doc.Clone()
	if err := clone.Root().Get("info").Set("name", "patched"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := doc.Root().Get("info").Get("name").String(); got != "original" {
		t.Fatalf("original mutated: %q", got)
	}
	if got := clone.Root().Get("info").Get("name").String(); got != "patched" {
		t.Fatalf("clone not patched: %q", got)
	}
}

func TestCanonicalIndentIsTwoSpaces(t *testing.T) {
	doc, err := Parse([]byte("root:\n    nested: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\n  nested: 1\n") {
		t.Fatalf("expected two-space indent, got:\n%s", out)
	}
}

func TestNewDocumentFromScratch(t *testing.T) {
	root := NewMap()
	http := NewMap()
	if err := root.Set("http", http); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := http.Set("method", "GET"); err != nil {
		t.Fatalf("set: %v", err)
	}
	out, err := NewDocument(root).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, "http:") || !strings.Contains(got, "method: GET") {
		t.Fatalf("output:\n%s", got)
	}
}
