// Package bru parses and prints Bruno's .bru file format.
//
// The parser is permissive on input and canonical on output: unknown blocks are
// kept verbatim, duplicate keys stay ordered pairs, and malformed input yields a
// partial AST plus an error that carries path, line and column. See
// docs/BRU_FORMAT.md for the grammar this package implements.
package bru

import "strconv"

// Kind identifies a lexical token class.
type Kind uint8

// Token kinds. The set is deliberately small: the format is line oriented, so
// most of the structure is carried by punctuation and newlines rather than by a
// rich token vocabulary.
const (
	// EOF marks the end of input.
	EOF Kind = iota
	// Name is a bare word: a block name, a key, or a list item.
	Name
	// Quoted is a "..." string with \" escapes decoded.
	Quoted
	// At introduces an annotation such as @description('x').
	At
	// LBrace opens a block body.
	LBrace
	// RBrace closes a block body.
	RBrace
	// LBracket opens a list value.
	LBracket
	// RBracket closes a list value.
	RBracket
	// Colon separates a key from a value.
	Colon
	// Comma separates values where the grammar allows it.
	Comma
	// Newline ends a line.
	Newline
	// TextBlockStart is the ''' opening a multiline value.
	TextBlockStart
)

// String returns the token kind's name, for tests and error messages.
func (k Kind) String() string {
	switch k {
	case EOF:
		return "EOF"
	case Name:
		return "name"
	case Quoted:
		return "quoted"
	case At:
		return "at"
	case LBrace:
		return "brace"
	case RBrace:
		return "brace"
	case LBracket:
		return "bracket"
	case RBracket:
		return "bracket"
	case Colon:
		return "colon"
	case Comma:
		return "comma"
	case Newline:
		return "newline"
	case TextBlockStart:
		return "text block"
	default:
		return "kind(" + strconv.Itoa(int(k)) + ")"
	}
}

// Pos is a position in a source file. Offset is a byte offset; Line and Column
// are one-based. Column counts bytes, which matches the editor's byte-based
// cursor handling and keeps spans cheap to compute.
type Pos struct {
	Offset int
	Line   int
	Column int
}

// Span is a half-open byte range [Start, End).
type Span struct {
	Start Pos
	End   Pos
}

// String renders a span as "line:column", the form used in error messages.
func (s Span) String() string {
	return strconv.Itoa(s.Start.Line) + ":" + strconv.Itoa(s.Start.Column)
}

// Token is a lexical token with its source text and span.
type Token struct {
	Kind Kind
	// Text is the decoded token value: escapes resolved for Quoted, otherwise
	// the raw source text.
	Text string
	Span Span
}

// mark records a lexer position so a speculative read can be undone.
type mark struct {
	off       int
	line, col int
}
