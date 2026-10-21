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
