package bru

import "testing"

func TestLexerTokens(t *testing.T) {
	src := "meta {\r\n  name: Get users\r\n  tags: [\r\n    smoke\r\n  ]\r\n}\r\n"
	l := newLexer("test.bru", []byte(src))

	want := []struct {
		kind Kind
		text string
		line int
		col  int
	}{
		{Name, "meta", 1, 1},
		{LBrace, "{", 1, 6},
		{Newline, "\n", 1, 7},
		{Name, "name", 2, 3},
		{Colon, ":", 2, 7},
		{Name, "Get", 2, 9},
		{Name, "users", 2, 13},
		{Newline, "\n", 2, 18},
		{Name, "tags", 3, 3},
		{Colon, ":", 3, 7},
		{LBracket, "[", 3, 9},
		{Newline, "\n", 3, 10},
		{Name, "smoke", 4, 5},
		{Newline, "\n", 4, 10},
		{RBracket, "]", 5, 3},
		{Newline, "\n", 5, 4},
		{RBrace, "}", 6, 1},
		{Newline, "\n", 6, 2},
		{EOF, "", 7, 1},
	}

	for i, w := range want {
		tok := l.next()
		if tok.Kind != w.kind || tok.Text != w.text {
			t.Fatalf("token %d = (%v, %q), want (%v, %q)", i, tok.Kind, tok.Text, w.kind, w.text)
		}
		if tok.Span.Start.Line != w.line || tok.Span.Start.Column != w.col {
			t.Fatalf("token %d %q at %d:%d, want %d:%d", i, tok.Text, tok.Span.Start.Line, tok.Span.Start.Column, w.line, w.col)
		}
	}
}

func TestLexerQuotedKey(t *testing.T) {
	l := newLexer("test.bru", []byte("  ~\"X-Weird: key\": value\n"))
	key, disabled, _ := l.readKeyRaw()
	if key != "X-Weird: key" || !disabled {
		t.Fatalf("key %q disabled %v", key, disabled)
	}
	l.skipHorizontal()
	if l.cur() != ':' {
		t.Fatalf("expected colon, got %q", l.cur())
	}
	l.advance(1)
	v, _ := l.readValueLine()
	if v != "value" {
		t.Fatalf("value %q", v)
	}
}

func TestLexerMultilineValueStripsFourSpaces(t *testing.T) {
	src := "key: '''\n    line one\n      indented\n  '''"
	l := newLexer("test.bru", []byte(src))
	l.readBare() // key
	l.skipHorizontal()
	l.advance(1) // colon
	l.skipHorizontal()
	text, _, err := l.readMultilineValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "line one\n  indented"; text != want {
		t.Fatalf("text %q, want %q", text, want)
	}
}

func TestLexerAnnotation(t *testing.T) {
	cases := []struct {
		src   string
		name  string
		arg   string
		hasAr bool
	}{
		{"@number\n", "number", "", false},
		{"@description('Page size')\n", "description", "Page size", true},
		{"@description(\"a \\\"b\\\"\")\n", "description", `a "b"`, true},
		{"@list(one,two)\n", "list", "one,two", true},
	}
	for _, tc := range cases {
		l := newLexer("test.bru", []byte(tc.src))
		ann, ok, _ := l.readAnnotation()
		if !ok {
			t.Fatalf("%q: not recognized as an annotation", tc.src)
		}
		if ann.Name != tc.name || ann.Arg != tc.arg || ann.HasArg != tc.hasAr {
			t.Fatalf("%q = %+v", tc.src, ann)
		}
	}
}

func TestLexerAnnotationIsKey(t *testing.T) {
	l := newLexer("test.bru", []byte("@localVar: 1\n"))
	if _, ok, _ := l.readAnnotation(); ok {
		t.Fatal("@localVar: 1 must not parse as an annotation")
	}
	key, disabled, _ := l.readKeyRaw()
	if key != "@localVar" || disabled {
		t.Fatalf("key %q disabled %v", key, disabled)
	}
}

func TestLexerTextBlock(t *testing.T) {
	src := "{\n  line one\n\n  line two\n}\n"
	l := newLexer("test.bru", []byte(src))
	l.advance(1)
	text, _, err := l.readTextBlock()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "  line one\n\n  line two"; text != want {
		t.Fatalf("text %q, want %q", text, want)
	}
	if l.cur() != '}' {
		t.Fatalf("lexer must stop on the closing brace, at %q", l.cur())
	}
}

func TestLexerEmptyTextBlock(t *testing.T) {
	for _, src := range []string{"{\n}\n", "{}\n"} {
		l := newLexer("test.bru", []byte(src))
		l.advance(1)
		text, _, err := l.readTextBlock()
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", src, err)
		}
		if text != "" {
			t.Fatalf("%q: text %q, want empty", src, text)
		}
		if l.cur() != '}' {
			t.Fatalf("%q: lexer at %q, want '}'", src, l.cur())
		}
	}
}

func TestLexerUnterminatedTextBlock(t *testing.T) {
	l := newLexer("test.bru", []byte("{\n  never closed\n"))
	l.advance(1)
	if _, _, err := l.readTextBlock(); err == nil {
		t.Fatal("expected an error for an unterminated text block")
	}
}

func TestLexerListBody(t *testing.T) {
	l := newLexer("test.bru", []byte("[\n  alpha\n  \"beta gamma\",\n  delta\n]\n"))
	l.advance(1)
	items, _, err := l.readListBody()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"alpha", "beta gamma", "delta"}
	if len(items) != len(want) {
		t.Fatalf("items %v, want %v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Fatalf("items %v, want %v", items, want)
		}
	}
}
