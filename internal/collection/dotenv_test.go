package collection

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		want     map[string]string
		warnings int
	}{
		{name: "simple pair", src: "FOO=bar", want: map[string]string{"FOO": "bar"}},
		{
			name: "blank lines and comments",
			src:  "\n# a comment\nFOO=bar\n\n   # indented comment\n",
			want: map[string]string{"FOO": "bar"},
		},
		{
			name: "multiple pairs keep numbers as text",
			src:  "FOO=bar\nBAZ=2\nBEEP=false\n",
			want: map[string]string{"FOO": "bar", "BAZ": "2", "BEEP": "false"},
		},
		{
			name: "quoted value keeps whitespace",
			src:  "SPACE=\"  value  \"\n",
			want: map[string]string{"SPACE": "  value  "},
		},
		{
			name: "unquoted value is trimmed",
			src:  "SPACE=  value  \n",
			want: map[string]string{"SPACE": "value"},
		},
		{
			name: "unquoted value keeps inner spaces",
			src:  "TEXT=a  b \n",
			want: map[string]string{"TEXT": "a  b"},
		},
		{
			name: "whitespace-only value is empty",
			src:  "EMPTY=   \n",
			want: map[string]string{"EMPTY": ""},
		},
		{
			name: "single quotes keep their content literally",
			src:  "RAW='a\\nb'\n",
			want: map[string]string{"RAW": `a\nb`},
		},
		{
			name: "double quotes expand newline and carriage return",
			src:  "TEXT=\"a\\nb\\rc\"\n",
			want: map[string]string{"TEXT": "a\nb\rc"},
		},
		{
			name: "double quotes keep other escapes literal",
			src:  `PATH="c:\\tmp\\"` + "\n",
			want: map[string]string{"PATH": `c:\\tmp\\`},
		},
		{
			// A trailing backslash must not swallow the closing quote.
			name: "single quoted value ending in a backslash",
			src:  "ROOT='C:\\'\nNEXT=1\n",
			want: map[string]string{"ROOT": `C:\`, "NEXT": "1"},
		},
		{
			name: "double quoted value ending in a backslash",
			src:  "ROOT=\"C:\\tmp\\\"\nNEXT=1\n",
			want: map[string]string{"ROOT": `C:\tmp\`, "NEXT": "1"},
		},
		{
			name: "escaped quote inside single quotes",
			src:  "KEY='a\\'b'\n",
			want: map[string]string{"KEY": `a\'b`},
		},
		{
			name: "escaped backslash before the closing quote",
			src:  "KEY='a\\\\'\n",
			want: map[string]string{"KEY": `a\\`},
		},
		{name: "backtick quotes", src: "CMD=`ls -la`\n", want: map[string]string{"CMD": "ls -la"}},
		{
			name: "multiline double quoted value",
			src:  "KEY=\"line one\nline two\"\nNEXT=1\n",
			want: map[string]string{"KEY": "line one\nline two", "NEXT": "1"},
		},
		{
			name: "multiline single quoted value",
			src:  "KEY='line one\nline two'\n",
			want: map[string]string{"KEY": "line one\nline two"},
		},
		{
			name: "comment after a closing quote",
			src:  "KEY=\"v\"  # note\n",
			want: map[string]string{"KEY": "v"},
		},
		{
			// dotenv falls back to the raw rest of the line, quotes included.
			name:     "text after a closing quote keeps the quotes",
			src:      "KEY=\"v\" tail\nNEXT=1\n",
			want:     map[string]string{"KEY": `"v" tail`, "NEXT": "1"},
			warnings: 1,
		},
		{
			name: "export prefix",
			src:  "export FOO=bar\n\texport\tBAZ=2\n",
			want: map[string]string{"FOO": "bar", "BAZ": "2"},
		},
		{name: "key starting with export is not the keyword", src: "exported=yes\n", want: map[string]string{"exported": "yes"}},
		{
			name: "colon separator needs whitespace after it",
			src:  "FOO: bar\nBAR:baz\n",
			want: map[string]string{"FOO": "bar"},

			warnings: 1,
		},
		{
			name:     "space before the colon is not a separator",
			src:      "FOO : bar\n",
			want:     map[string]string{},
			warnings: 1,
		},
		{name: "tab after the colon", src: "FOO:\tbar\n", want: map[string]string{"FOO": "bar"}},
		{name: "spaces before an equals sign", src: "FOO   = bar\n", want: map[string]string{"FOO": "bar"}},
		{
			name: "inline comment ends an unquoted value",
			src:  "FOO=bar # comment\nBAZ=2#tail\n",
			want: map[string]string{"FOO": "bar", "BAZ": "2"},
		},
		{name: "hash inside quotes is literal", src: "FOO=\"a#b\"\n", want: map[string]string{"FOO": "a#b"}},
		{name: "empty value", src: "EMPTY=\n", want: map[string]string{"EMPTY": ""}},
		{name: "line without a separator", src: "EMPTY2\n", want: map[string]string{}, warnings: 1},
		{
			name: "keys allow dots and hyphens",
			src:  "api.url=https://a\nx-token=abc\n",
			want: map[string]string{"api.url": "https://a", "x-token": "abc"},
		},
		{
			name: "duplicate key keeps the last value",
			src:  "A=1\nB=2\nA=3\n",
			want: map[string]string{"A": "3", "B": "2"},
		},
		{name: "CRLF and CR line endings", src: "A=1\r\nB=2\rC=3\n", want: map[string]string{"A": "1", "B": "2", "C": "3"}},
		{name: "byte order mark is stripped", src: "\ufeffA=1\n", want: map[string]string{"A": "1"}},
		{name: "value may contain equals signs", src: "URL=https://a?x=1&y=2\n", want: map[string]string{"URL": "https://a?x=1&y=2"}},
		{name: "only whitespace", src: " \n\t\n", want: map[string]string{}},
		{name: "empty file", src: "", want: map[string]string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := ParseDotEnv(".env", []byte(tc.src))
			got := env.Map()
			if len(got) != len(tc.want) {
				t.Fatalf("vars = %#v, want %#v", got, tc.want)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatalf("%s = %q, want %q (all: %#v)", key, got[key], want, got)
				}
			}
			if len(env.Warnings) != tc.warnings {
				t.Fatalf("warnings = %v, want %d", env.Warnings, tc.warnings)
			}
		})
	}
}

func TestParseDotEnvKeepsFileOrder(t *testing.T) {
	env := ParseDotEnv(".env", []byte("ZED=1\nALPHA=2\nZED=3\nMID=4\n"))
	var names []string
	for _, v := range env.Vars {
		names = append(names, v.Name)
	}
	if got := strings.Join(names, ","); got != "ZED,ALPHA,MID" {
		t.Fatalf("order = %s, want file order with the duplicate collapsed in place", got)
	}
	if env.Vars[0].Value != "3" {
		t.Fatalf("ZED = %q, want the last value", env.Vars[0].Value)
	}
}

func TestParseDotEnvWarnsInsteadOfFailing(t *testing.T) {
	env := ParseDotEnv(".env", []byte("GOOD=1\nnonsense line\nALSO=2\n"))
	if len(env.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one", env.Warnings)
	}
	if !strings.Contains(env.Warnings[0].Error(), ".env:2") {
		t.Fatalf("warning has no line number: %v", env.Warnings[0])
	}
	if got := env.Map(); len(got) != 2 || got["ALSO"] != "2" {
		t.Fatalf("a bad line must not stop the rest: %#v", got)
	}
}

func TestParseDotEnvWarningLineNumbers(t *testing.T) {
	src := "A=1\n\n# comment\nbad line\nB=\"unterminated\nC=3\n"
	env := ParseDotEnv(".env", []byte(src))
	if len(env.Warnings) != 2 {
		t.Fatalf("warnings = %v, want two", env.Warnings)
	}
	if !strings.Contains(env.Warnings[0].Error(), ".env:4") {
		t.Fatalf("first warning line = %v", env.Warnings[0])
	}
	// The unterminated quote started on line 5, even though the scan reached EOF.
	if !strings.Contains(env.Warnings[1].Error(), ".env:5") {
		t.Fatalf("second warning line = %v", env.Warnings[1])
	}
}

func TestParseDotEnvUnterminatedQuote(t *testing.T) {
	env := ParseDotEnv(".env", []byte("KEY=\"never closed\nREST=1\n"))
	if len(env.Warnings) != 1 || !strings.Contains(env.Warnings[0].Error(), "closing quote") {
		t.Fatalf("warnings = %v", env.Warnings)
	}
	// dotenv's fallback keeps the rest of the line, quote included, and the next
	// line still parses.
	if got, ok := env.Lookup("KEY"); !ok || got != `"never closed` {
		t.Fatalf("KEY = %q, ok %v", got, ok)
	}
	if got, ok := env.Lookup("REST"); !ok || got != "1" {
		t.Fatalf("REST = %q, ok %v", got, ok)
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	env, err := LoadDotEnv(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if env != nil {
		t.Fatalf("env = %+v, want nil", env)
	}
}

func TestLoadDotEnvStopsOnCancelledContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DotEnvFile), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadDotEnv(ctx, dir); err == nil {
		t.Fatal("a cancelled context must stop the read")
	}
}

func TestLoadDotEnvRejectsADirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, DotEnvFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDotEnv(context.Background(), dir); err == nil {
		t.Fatal("a directory named .env must be an error")
	}
}

func TestLoadReadsOnlyDotEnv(t *testing.T) {
	dir := writeYAMLTree(t, map[string]string{
		"opencollection.yml": "opencollection: '1.0.0'\ninfo:\n  name: dotenv\n",
		".env":               "SHARED=collection\nONLY_HERE=yes\n",
		".env.local":         "SHARED=local\nLEAKED=yes\n",
		".env.production":    "LEAKED=yes\n",
	})

	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.DotEnv == nil {
		t.Fatal(".env was not read")
	}
	if got := c.DotEnv.Map()["SHARED"]; got != "collection" {
		t.Fatalf("SHARED = %q", got)
	}
	if _, ok := c.DotEnv.Lookup("LEAKED"); ok {
		t.Fatal(".env.local must be inert")
	}
	if len(c.Warnings) != 0 {
		t.Fatalf("warnings = %v", c.Warnings)
	}
}

func TestLoadDotEnvWarningsReachTheCollection(t *testing.T) {
	dir := writeYAMLTree(t, map[string]string{
		"opencollection.yml": "opencollection: '1.0.0'\ninfo:\n  name: broken-env\n",
		".env":               "GOOD=1\nthis is not an assignment\n",
	})
	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Warnings) != 1 || !strings.Contains(c.Warnings[0].Error(), "dotenv") {
		t.Fatalf("warnings = %v", c.Warnings)
	}
	if len(c.DotEnv.Warnings) != 1 {
		t.Fatalf("the parse warnings should stay on the file too: %v", c.DotEnv.Warnings)
	}
}

func TestBuildProcessEnvPrecedence(t *testing.T) {
	workspace := ParseDotEnv("workspace/.env", []byte("SHARED=workspace\nWS_ONLY=yes\n"))
	collectionEnv := ParseDotEnv("collection/.env", []byte("SHARED=collection\nCOL_ONLY=yes\n"))

	got := BuildProcessEnv([]string{
		"SHARED=os",
		"WS_ONLY=os",
		"OS_ONLY=yes",
		"MALFORMED",
	}, workspace, collectionEnv)

	want := map[string]string{
		"SHARED":   "collection",
		"WS_ONLY":  "yes",
		"COL_ONLY": "yes",
		"OS_ONLY":  "yes",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d variables, want %d: %#v", len(got), len(want), got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %q, want %q (all: %#v)", key, got[key], value, got)
		}
	}
	if _, ok := got["MALFORMED"]; ok {
		t.Fatal("an entry without '=' must be skipped")
	}
}

func TestCollectionProcessEnv(t *testing.T) {
	c := &Collection{DotEnv: ParseDotEnv(".env", []byte("TOKEN=from-file\n"))}
	got := c.ProcessEnv([]string{"TOKEN=from-os", "HOME=/tmp"}, nil)
	if got["TOKEN"] != "from-file" {
		t.Fatalf("TOKEN = %q, want the collection file to win", got["TOKEN"])
	}
	if got["HOME"] != "/tmp" {
		t.Fatalf("OS variables must survive: %#v", got)
	}

	empty := &Collection{}
	if got := empty.ProcessEnv([]string{"HOME=/tmp"}, nil); got["HOME"] != "/tmp" {
		t.Fatalf("no .env must still expose the OS environment: %#v", got)
	}
}

func TestDotEnvNilReceiver(t *testing.T) {
	var env *DotEnv
	if got := env.Map(); len(got) != 0 {
		t.Fatalf("Map = %#v", got)
	}
	if _, ok := env.Lookup("A"); ok {
		t.Fatal("Lookup on nil must report false")
	}
}

func TestDotEnvIsNotAVariableScope(t *testing.T) {
	dir := writeYAMLTree(t, map[string]string{
		"opencollection.yml": "opencollection: '1.0.0'\ninfo:\n  name: scopes\n",
		"a.yml":              "info:\n  name: a\n  type: http\nhttp:\n  url: https://a\n",
		".env":               "SECRET_ENV_NAME=value\n",
	})
	c, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	node := findNode(t, c, "a.yml")
	for _, v := range ResolvedVarsPre(node) {
		if v.Name == "SECRET_ENV_NAME" {
			t.Fatalf("dotenv leaked into the variable chain: %+v", v)
		}
	}
	if got := c.ProcessEnv(nil, nil)["SECRET_ENV_NAME"]; got != "value" {
		t.Fatalf("process.env lookup = %q", got)
	}
}

// FuzzParseDotEnv pins two invariants: parsing always terminates, and the map
// view agrees with the ordered list (duplicates are collapsed, never dropped).
func FuzzParseDotEnv(f *testing.F) {
	seeds := []string{
		"A=1\n# c\nexport B='x'\n",
		"A=\"multi\nline\"\n",
		"A='\\\n",
		"KEY=\"v\" tail\n",
		"K: v\n",
		"\ufeffA=1\r\nB=2\r",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		env := ParseDotEnv(".env", src)
		values := env.Map()
		if len(values) != len(env.Vars) {
			t.Fatalf("map has %d entries, list has %d", len(values), len(env.Vars))
		}
		for _, v := range env.Vars {
			if v.Name == "" {
				t.Fatalf("empty name in %#v", env.Vars)
			}
			if got := values[v.Name]; got != v.Value {
				t.Fatalf("map[%q] = %q, list says %q", v.Name, got, v.Value)
			}
		}
	})
}

// TestParseDotEnvMatchesReference replays a corpus of inputs against the output
// npm dotenv 16.4.7 produces for them (testdata/dotenv/reference.json, captured
// from `dotenv.parse`). It is the parity check for the grammar: a change that
// starts interpreting a value differently shows up as a mismatch here rather
// than in a user's collection.
//
// The corpus stays on one line per entry, because apigo deliberately requires a
// key, separator and value to share a line, while dotenv's pattern lets
// whitespace (including a newline) sit between them.
func TestParseDotEnvMatchesReference(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "dotenv", "reference.json"))
	if err != nil {
		t.Fatalf("read reference corpus: %v", err)
	}
	var cases []struct {
		Input  string            `json:"input"`
		Dotenv map[string]string `json:"dotenv"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode reference corpus: %v", err)
	}
	if len(cases) < 40 {
		t.Fatalf("corpus shrank to %d cases", len(cases))
	}

	for _, tc := range cases {
		got := ParseDotEnv(".env", []byte(tc.Input)).Map()
		if !maps.Equal(got, tc.Dotenv) {
			t.Errorf("input %q\n  dotenv: %#v\n  apigo : %#v", tc.Input, tc.Dotenv, got)
		}
	}
}
