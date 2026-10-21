package bru

import (
	"bytes"
	"strings"
)

// EnvFile is a parsed environment file (environments/<name>.bru).
//
// Environment files use a separate grammar from requests: a vars dictionary, a
// vars:secret name list, optional external secret providers, an extends
// reference and a colour hint.
type EnvFile struct {
	// Path is the source path.
	Path string
	// Variables is the vars dictionary, in source order.
	Variables []Pair
	// Secrets lists secret variable names. Values live outside the file.
	Secrets []string
	// Extends lists parent environment names.
	Extends []string
	// Color is the UI hint written by Bruno, e.g. "#4caf50".
	Color string
	// External holds vars:externalsecrets:<provider> blocks, preserved whether or
	// not a provider is implemented.
	External []*ExternalSecrets
	// Unknown holds directives this version does not understand, verbatim.
	Unknown []*RawBlock
	// Warnings holds recoverable parse problems.
	Warnings []*Error
}

// ExternalSecrets is a vars:externalsecrets:<provider> block.
type ExternalSecrets struct {
	Provider string
	Pairs    []Pair
	Span     Span
}

// BlockName implements Block.
func (e *ExternalSecrets) BlockName() string { return "vars:externalsecrets:" + e.Provider }

// BlockSpan implements Block.
func (e *ExternalSecrets) BlockSpan() Span { return e.Span }

// ParseEnv parses an environment file. Unknown directives are preserved.
func ParseEnv(path string, src []byte) (*EnvFile, error) {
	if bytes.HasPrefix(src, utf8BOM) {
		src = src[len(utf8BOM):]
	}
	p := &parser{lx: newLexer(path, src), src: src, file: &File{Path: path}}
	env := &EnvFile{Path: path}

	for {
		p.lx.skipBlankLines()
		if p.lx.eof() {
			break
		}
		start := p.lx.pos()
		name, nameSpan := p.lx.readBlockName()
		if name == "" {
			p.fail(start, "expected an environment directive")
			p.lx.skipLine()
			continue
		}
		p.lx.skipHorizontal()

		switch {
		case name == "vars" && p.lx.cur() == '{':
			d := p.parseDictInto(name, start)
			env.Variables = append(env.Variables, d.Pairs...)
		case name == "vars:secret" && p.lx.cur() == '[':
			p.lx.advance(1)
			items, _, err := p.lx.readListBody()
			if err != nil {
				p.errs = append(p.errs, err.(*Error))
			}
			env.Secrets = append(env.Secrets, items...)
			p.finishLine()
		case strings.HasPrefix(name, "vars:externalsecrets:") && p.lx.cur() == '{':
			provider := strings.TrimPrefix(name, "vars:externalsecrets:")
			d := p.parseDictInto(name, start)
			env.External = append(env.External, &ExternalSecrets{Provider: provider, Pairs: d.Pairs, Span: d.Span})
		case strings.HasPrefix(name, "extends:"):
			value := strings.TrimSpace(strings.TrimPrefix(name, "extends:"))
			if value == "" {
				value, _ = p.lx.readValueLine()
			}
			if value != "" {
				env.Extends = append(env.Extends, value)
			}
			p.finishLine()
		case name == "extends" && p.lx.cur() == '[':
			p.lx.advance(1)
			items, _, err := p.lx.readListBody()
			if err != nil {
				p.errs = append(p.errs, err.(*Error))
			}
			env.Extends = append(env.Extends, items...)
			p.finishLine()
		case strings.HasPrefix(name, "color:"):
			env.Color = strings.TrimSpace(strings.TrimPrefix(name, "color:"))
			if env.Color == "" {
				env.Color, _ = p.lx.readValueLine()
			}
			p.finishLine()
		default:
			raw := p.captureRawLine(start, nameSpan)
			env.Unknown = append(env.Unknown, &RawBlock{Name: name, Raw: raw, Span: p.lx.span(start)})
		}
	}

	if len(p.errs) > 0 {
		return env, ErrorList(p.errs)
	}
	return env, nil
}

// finishLine consumes trailing spaces and one line break.
func (p *parser) finishLine() {
	p.lx.skipHorizontal()
	if p.lx.atNewline() {
		p.lx.skipNewline()
	}
}

// parseDictInto parses a dictionary block into a fresh DictBlock without adding
// it to the request file's block list.
func (p *parser) parseDictInto(name string, start Pos) *DictBlock {
	before := len(p.file.Blocks)
	p.parseDictBlock(name, start)
	d, _ := p.file.Blocks[before].(*DictBlock)
	p.file.Blocks = p.file.Blocks[:before]
	if d == nil {
		return &DictBlock{Name: name, Span: p.lx.span(start)}
	}
	return d
}

// captureRawLine preserves an unrecognized directive line.
func (p *parser) captureRawLine(start Pos, nameSpan Span) string {
	if p.lx.atNewline() || p.lx.eof() {
		p.lx.skipNewline()
		return string(p.src[nameSpan.Start.Offset:nameSpan.End.Offset])
	}
	_, lineSpan := p.lx.readLine()
	raw := strings.TrimRight(string(p.src[start.Offset:lineSpan.End.Offset]), "\r")
	p.lx.skipNewline()
	return raw
}

// FormatEnv renders an environment file in canonical form.
func FormatEnv(env *EnvFile) []byte {
	var b strings.Builder

	if len(env.Extends) == 1 {
		b.WriteString("extends: ")
		b.WriteString(env.Extends[0])
		b.WriteString("\n")
	} else if len(env.Extends) > 1 {
		b.WriteString("extends [\n")
		for _, name := range env.Extends {
			b.WriteString("  ")
			b.WriteString(quoteExtendsName(name))
			b.WriteString(",\n")
		}
		b.WriteString("]\n")
	}

	b.WriteString("vars {\n")
	writePairs(&b, env.Variables)
	b.WriteString("}\n")

	if len(env.Secrets) > 0 {
		b.WriteString("vars:secret [\n")
		for i, name := range env.Secrets {
			b.WriteString("  ")
			b.WriteString(name)
			if i < len(env.Secrets)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("]\n")
	}

	for _, ext := range env.External {
		b.WriteString("vars:externalsecrets:")
		b.WriteString(ext.Provider)
		b.WriteString(" {\n")
		writePairs(&b, ext.Pairs)
		b.WriteString("}\n")
	}

	if env.Color != "" {
		b.WriteString("color: ")
		b.WriteString(env.Color)
		b.WriteString("\n")
	}

	for _, raw := range env.Unknown {
		b.WriteString(strings.TrimRight(raw.Raw, "\n"))
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// writePairs writes a pair list without the enclosing block: enabled pairs first,
// then disabled ones, matching the request printer.
func writePairs(b *strings.Builder, pairs []Pair) {
	enabled := make([]Pair, 0, len(pairs))
	disabled := make([]Pair, 0, len(pairs))
	for _, p := range pairs {
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
}

// quoteExtendsName quotes a parent environment name when the bare form would not
// survive a re-parse.
func quoteExtendsName(name string) string {
	if strings.TrimSpace(name) != name || strings.ContainsAny(name, `[],"`) {
		return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
	}
	return name
}
