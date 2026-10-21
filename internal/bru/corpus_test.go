package bru

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBrunoCorpus parses every vendored Bruno fixture and checks the format
// invariants: parsing succeeds, formatting is idempotent, and the canonical
// output still describes the same file.
func TestBrunoCorpus(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "bruno", "*.bru"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no Bruno fixtures found")
	}

	for _, path := range fixtures {
		name := filepath.Base(path)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if strings.HasSuffix(name, "-env.bru") {
			env, err := ParseEnv(path, src)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			printed := FormatEnv(env)
			again, err := ParseEnv(path, printed)
			if err != nil {
				t.Fatalf("%s: reparse: %v", name, err)
			}
			if string(FormatEnv(again)) != string(printed) {
				t.Fatalf("%s: environment formatting is not idempotent", name)
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
			t.Fatalf("%s: AST changed across a round trip", name)
		}
		if again := Format(second); string(again) != string(printed) {
			t.Fatalf("%s: formatting is not idempotent", name)
		}
	}
}
