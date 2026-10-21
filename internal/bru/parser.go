package bru

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// utf8BOM is the byte order mark Bruno tolerates and never writes back.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// dictBlockNames lists the dictionary blocks of the request, folder and
// collection grammars. Names starting with "auth:" are dictionary blocks too;
// see isDictBlock.
var dictBlockNames = map[string]bool{
	"meta":     true,
	"app":      true,
	"settings": true,
	"http":     true,
	"get":      true,
	"post":     true,
	"put":      true,
	"patch":    true,
	"delete":   true,
	"head":     true,
	"options":  true,
	"trace":    true,
	"connect":  true,
	"grpc":     true,
	"ws":       true,
	"headers":  true,
	"metadata": true,
	"query":    true,

	"params:path":  true,
	"params:query": true,

	"vars:pre-request":   true,
	"vars:post-response": true,
	"assert":             true,

	"body:form-urlencoded": true,
	"body:multipart-form":  true,
	"body:file":            true,
	"body:grpc":            true,
	"body:ws":              true,
}

// textBlockNames lists the blocks whose body is raw text.
var textBlockNames = map[string]bool{
	"body":              true,
	"body:json":         true,
	"body:text":         true,
	"body:xml":          true,
	"body:sparql":       true,
	"body:graphql":      true,
	"body:graphql:vars": true,

	"script:pre-request":                true,
	"script:post-response":              true,
	"script:grpc:before-call-start":     true,
	"script:grpc:before-message-send":   true,
	"script:grpc:after-message-receive": true,
	"script:grpc:after-call-end":        true,

	"tests": true,
	"docs":  true,
	// Content is preserved verbatim; the example grammar is parsed separately.
	"example": true,
}

// isDictBlock reports whether a block name denotes a dictionary block.
func isDictBlock(name string) bool {
	if dictBlockNames[name] {
		return true
	}
	return strings.HasPrefix(name, "auth:")
}

// isTextBlock reports whether a block name denotes a raw text block.
func isTextBlock(name string) bool {
	if textBlockNames[name] {
		return true
	}
	return strings.HasPrefix(name, "script:")
}

// Parse parses a .bru source file. Unknown blocks are preserved verbatim.
//
// The returned File is always non-nil: malformed input yields a partial AST plus
// an ErrorList, so the caller can show what parsed and what did not. Warnings
// (for example invalid UTF-8) do not fail the parse.
func Parse(path string, src []byte) (*File, error) {
	if bytes.HasPrefix(src, utf8BOM) {
		src = src[len(utf8BOM):]
	}
	p := &parser{lx: newLexer(path, src), src: src, file: &File{Path: path}}
	if !utf8.Valid(src) {
		p.file.add(path, Pos{Line: 1, Column: 1}, "file is not valid UTF-8; invalid bytes are preserved")
	}
	p.parseFile()
	if len(p.errs) > 0 {
		return p.file, ErrorList(p.errs)
	}
	return p.file, nil
}

type parser struct {
	lx   *lexer
	src  []byte
	file *File
	errs []*Error
}

func (p *parser) fail(pos Pos, format string, args ...any) {
	p.errs = append(p.errs, &Error{Path: p.file.Path, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (p *parser) parseFile() {
	for {
		p.lx.skipBlankLines()
		if p.lx.eof() {
			return
		}
		start := p.lx.pos()
		name, nameSpan := p.lx.readBlockName()
		if name == "" {
			p.fail(start, "expected a block name")
			p.lx.skipLine()
			continue
		}
		p.lx.skipHorizontal()

		switch {
		case p.lx.cur() == '{':
			p.parseBraceBlock(name, start)
		case p.lx.cur() == '[':
			p.parseRawBlock(name, start, ']')
		default:
			// A ``key: value`` line at block level is not part of the request
			// grammar. Keep it verbatim instead of losing it.
			p.parseRawLine(name, start, nameSpan)
		}
	}
}

// parseBraceBlock dispatches on the block name. Unknown names are preserved.
func (p *parser) parseBraceBlock(name string, start Pos) {
	switch {
	case isTextBlock(name):
		p.parseTextBlock(name, start)
	case isDictBlock(name):
		p.parseDictBlock(name, start)
	default:
		p.parseRawBlock(name, start, '}')
	}
}

func (p *parser) parseTextBlock(name string, start Pos) {
	p.lx.advance(1) // '{'
	text, _, err := p.lx.readTextBlock()
	if err != nil {
		p.errs = append(p.errs, err.(*Error))
	}
	if p.lx.cur() == '}' {
		p.lx.advance(1)
	} else {
		p.fail(p.lx.pos(), "unterminated %s block", name)
	}
	p.file.Blocks = append(p.file.Blocks, &TextBlock{
		Name:    name,
		Text:    text,
		Span:    p.lx.span(start),
		Example: name == "example",
	})
}

func (p *parser) parseDictBlock(name string, start Pos) {
	p.lx.advance(1) // '{'
	blk := &DictBlock{Name: name, AssertSyntax: name == "assert"}

	for {
		p.lx.skipBlankLines()
		if p.lx.eof() {
			p.fail(start, "unterminated %s block", name)
			break
		}
		if p.lx.cur() == '}' {
			p.lx.advance(1)
			break
		}

		var anns []Annotation
		for p.lx.cur() == '@' {
			ann, ok, _ := p.lx.readAnnotation()
			if !ok {
				break
			}
			anns = append(anns, ann)
			p.lx.skipHorizontal()
			if p.lx.atNewline() {
				p.lx.skipNewline()
			}
			p.lx.skipBlankLines()
		}

		pairStart := p.lx.pos()
		key, disabled, _ := p.lx.readKeyRaw()
		p.lx.skipHorizontal()
		if p.lx.cur() != ':' {
			p.fail(p.lx.pos(), "expected ':' after key %q", key)
			p.lx.skipLine()
			continue
		}
		p.lx.advance(1)
		p.lx.skipHorizontal()

		value := p.readValue()
		blk.Pairs = append(blk.Pairs, Pair{
			Key:         key,
			Disabled:    disabled,
			Annotations: anns,
			Value:       value,
			Span:        p.lx.span(pairStart),
		})

		p.lx.skipHorizontal()
		switch {
		case p.lx.eof():
		case p.lx.atNewline():
			p.lx.skipNewline()
		default:
			p.fail(p.lx.pos(), "unexpected trailing content after value")
			p.lx.skipLine()
		}
	}

	blk.Span = p.lx.span(start)
	p.file.Blocks = append(p.file.Blocks, blk)
}

// readValue reads a pair's right-hand side.
func (p *parser) readValue() Value {
	if p.lx.cur() == '[' && p.listFollows() {
		p.lx.advance(1)
		items, _, err := p.lx.readListBody()
		if err != nil {
			p.errs = append(p.errs, err.(*Error))
		}
		return Value{Kind: ValueList, Items: items}
	}
	if p.lx.hasPrefix("'''") {
		text, _, err := p.lx.readMultilineValue()
		if err != nil {
			p.errs = append(p.errs, err.(*Error))
		}
		return Value{Kind: ValueMultiline, Text: text}
	}
	text, _ := p.lx.readValueLine()
	return Value{Kind: ValueString, Text: text}
}

// listFollows reports whether a '[' starts a list value rather than a literal.
// The grammar requires the list body to start on the next line.
func (p *parser) listFollows() bool {
	m := p.lx.mark()
	p.lx.advance(1)
	p.lx.skipHorizontal()
	ok := p.lx.atNewline() || p.lx.eof() || p.lx.cur() == ']'
	p.lx.reset(m)
	return ok
}

// parseRawBlock preserves an unknown block byte for byte.
func (p *parser) parseRawBlock(name string, start Pos, closer byte) {
	rawStart := start.Offset
	p.lx.advance(1) // opening brace or bracket
	for {
		if p.lx.eof() {
			p.fail(start, "unterminated %s block", name)
			break
		}
		if p.lx.cur() == closer && p.lx.col == 1 {
			p.lx.advance(1)
			break
		}
		p.lx.advance(1)
	}
	rawEnd := min(p.lx.off, len(p.src))
	p.file.Blocks = append(p.file.Blocks, &RawBlock{
		Name: name,
		Raw:  string(p.src[rawStart:rawEnd]),
		Span: p.lx.span(start),
	})
}

// parseRawLine preserves a single unrecognized line.
func (p *parser) parseRawLine(name string, start Pos, nameSpan Span) {
	if p.lx.atNewline() || p.lx.eof() {
		raw := string(p.src[nameSpan.Start.Offset:nameSpan.End.Offset])
		p.file.Blocks = append(p.file.Blocks, &RawBlock{Name: name, Raw: raw, Span: nameSpan})
		p.lx.skipNewline()
		return
	}
	_, lineSpan := p.lx.readLine()
	raw := strings.TrimRight(string(p.src[start.Offset:lineSpan.End.Offset]), "\r")
	p.file.Blocks = append(p.file.Blocks, &RawBlock{Name: name, Raw: raw, Span: p.lx.span(start)})
	p.lx.skipNewline()
}

// readBlockName reads a block name. Block names may contain ':' (params:query,
// auth:oauth2) but never whitespace or an opening brace or bracket.
func (l *lexer) readBlockName() (string, Span) {
	start := l.pos()
	from := l.off
	for !l.eof() {
		switch l.cur() {
		case ' ', '\t', '\r', '\n', '{', '[':
			return string(l.src[from:l.off]), l.span(start)
		}
		l.advance(1)
	}
	return string(l.src[from:l.off]), l.span(start)
}

// skipLine consumes the rest of the line, including the line break.
func (l *lexer) skipLine() {
	for !l.eof() && !l.atNewline() {
		l.advance(1)
	}
	l.skipNewline()
}
