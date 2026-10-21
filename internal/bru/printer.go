package bru

import (
	"sort"
	"strings"
)

// Block ranks for canonical output. They follow docs/BRU_FORMAT.md §7: known
// blocks in a stable order, then unknown blocks in source order, then examples.
const (
	rankMeta = 10 + iota
	rankMethod
	rankParamsQuery
	rankParamsPath
	rankQuery
	rankHeaders
	rankMetadata
	rankAuth
	rankBody
	rankVarsPre
	rankVarsPost
	rankAssert
	rankScript
	rankTests
	rankApp
	rankSettings
	rankDocs
	rankUnknown
	rankExample
)

// methodBlocks are the HTTP verb block names, plus the custom `http` block.
var methodBlocks = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true,
	"head": true, "options": true, "trace": true, "connect": true, "http": true,
}

// scriptOrder positions script blocks in execution order.
var scriptOrder = map[string]int{
	"script:pre-request":                0,
	"script:post-response":              1,
	"script:grpc:before-call-start":     2,
	"script:grpc:before-message-send":   3,
	"script:grpc:after-message-receive": 4,
	"script:grpc:after-call-end":        5,
}

// blockRank returns the canonical position of a block.
func blockRank(b Block) int {
	name := b.BlockName()
	switch {
	case name == "meta":
		return rankMeta
	case methodBlocks[name]:
		return rankMethod
	case name == "params:query":
		return rankParamsQuery
	case name == "params:path":
		return rankParamsPath
	case name == "query":
		return rankQuery
	case name == "headers":
		return rankHeaders
	case name == "metadata":
		return rankMetadata
	case name == "auth" || strings.HasPrefix(name, "auth:"):
		return rankAuth
	case strings.HasPrefix(name, "body"):
		return rankBody
	case name == "vars:pre-request":
		return rankVarsPre
	case name == "vars:post-response":
		return rankVarsPost
	case name == "assert":
		return rankAssert
	case strings.HasPrefix(name, "script:"):
		return rankScript
	case name == "tests":
		return rankTests
	case name == "app":
		return rankApp
	case name == "settings":
		return rankSettings
	case name == "docs":
		return rankDocs
	case name == "example":
		return rankExample
	default:
		return rankUnknown
	}
}

// secondaryRank orders blocks that share a rank.
func secondaryRank(b Block) int {
	if s, ok := scriptOrder[b.BlockName()]; ok {
		return s
	}
	return 0
}

// Format renders a file in canonical form: blocks in the documented order, two
// space indentation, LF line endings, one trailing newline.
//
// Format is deterministic: formatting the output again yields the same bytes.
func Format(f *File) []byte {
	var b strings.Builder

	blocks := make([]Block, len(f.Blocks))
	copy(blocks, f.Blocks)
	sort.SliceStable(blocks, func(i, j int) bool {
		ri, rj := blockRank(blocks[i]), blockRank(blocks[j])
		if ri != rj {
			return ri < rj
		}
		return secondaryRank(blocks[i]) < secondaryRank(blocks[j])
	})

	for i, blk := range blocks {
		if i > 0 {
			b.WriteString("\n")
		}
		writeBlock(&b, blk)
	}
	return []byte(b.String())
}

// FormatString is Format as a string.
func FormatString(f *File) string { return string(Format(f)) }

func writeBlock(b *strings.Builder, blk Block) {
	switch v := blk.(type) {
	case *DictBlock:
		writeDict(b, v)
	case *TextBlock:
		b.WriteString(v.Name)
		b.WriteString(" {\n")
		b.WriteString(v.Text)
		b.WriteString("\n}\n")
	case *RawBlock:
		raw := strings.TrimRight(v.Raw, "\n")
		b.WriteString(raw)
		b.WriteString("\n")
	default:
		// A nil or unknown implementation prints nothing rather than panicking
		// on a hand-built AST.
	}
}

func writeDict(b *strings.Builder, d *DictBlock) {
	b.WriteString(d.Name)
	b.WriteString(" {\n")

	enabled := make([]Pair, 0, len(d.Pairs))
	disabled := make([]Pair, 0, len(d.Pairs))
	for _, p := range d.Pairs {
		if p.Disabled {
			disabled = append(disabled, p)
		} else {
			enabled = append(enabled, p)
		}
	}
	for _, p := range enabled {
		writePair(b, p, false)
	}
	for _, p := range disabled {
		writePair(b, p, true)
	}
	b.WriteString("}\n")
}

func writePair(b *strings.Builder, p Pair, disabled bool) {
	for _, a := range p.Annotations {
		b.WriteString("  ")
		b.WriteString(formatAnnotation(a))
		b.WriteString("\n")
	}

	b.WriteString("  ")
	if disabled {
		b.WriteString("~")
	}
	b.WriteString(quoteKey(p.Key))
	b.WriteString(": ")
	writeValue(b, p.Value)
}

func writeValue(b *strings.Builder, v Value) {
	switch v.Kind {
	case ValueList:
		b.WriteString("[\n")
		for _, item := range v.Items {
			b.WriteString("    ")
			b.WriteString(item)
			b.WriteString("\n")
		}
		b.WriteString("  ]\n")
	case ValueMultiline:
		b.WriteString("'''\n")
		for _, line := range strings.Split(v.Text, "\n") {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("  '''\n")
	default:
		b.WriteString(v.Text)
		b.WriteString("\n")
	}
}

// quoteKey quotes keys that would otherwise change meaning on re-parse.
func quoteKey(key string) string {
	needsQuote := key == "" ||
		strings.TrimSpace(key) != key ||
		strings.ContainsAny(key, `:"{}[]`) ||
		strings.HasPrefix(key, "~")
	if !needsQuote {
		return key
	}
	return `"` + strings.ReplaceAll(key, `"`, `\"`) + `"`
}

// formatAnnotation renders an annotation, choosing the quoting that round-trips.
func formatAnnotation(a Annotation) string {
	if !a.HasArg {
		return "@" + a.Name
	}
	if strings.Contains(a.Arg, "\n") {
		var b strings.Builder
		b.WriteString("@")
		b.WriteString(a.Name)
		b.WriteString("('''\n")
		for _, line := range strings.Split(a.Arg, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("''')")
		return b.String()
	}
	if strings.Contains(a.Arg, "'") {
		escaped := strings.ReplaceAll(a.Arg, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return "@" + a.Name + `("` + escaped + `")`
	}
	return "@" + a.Name + "('" + a.Arg + "')"
}
