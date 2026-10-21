package bru

import "strings"

// File is a parsed .bru file: an ordered list of blocks.
//
// Block order is semantically irrelevant on input; the printer emits a canonical
// order. Blocks the parser does not understand are kept verbatim as RawBlock so
// a load/save cycle never loses data.
type File struct {
	// Path is the source path, used in error messages and diagnostics.
	Path string
	// Blocks is the file's blocks in source order.
	Blocks []Block
	// Warnings holds recoverable parse problems. A file with warnings still
	// produces a usable AST.
	Warnings []*Error
}

// Block is one top-level construct in a .bru file.
type Block interface {
	// BlockName is the name written before the opening brace or bracket.
	BlockName() string
	// BlockSpan is the block's source range, including its header.
	BlockSpan() Span
}

// ValueKind distinguishes the three value shapes a pair can hold.
type ValueKind uint8

// Value shapes.
const (
	// ValueString is a trimmed single-line value.
	ValueString ValueKind = iota
	// ValueMultiline is a ''' block value with four-space indentation removed.
	ValueMultiline
	// ValueList is a bracketed list of items.
	ValueList
)

// Value is the right-hand side of a pair.
type Value struct {
	Kind ValueKind
	// Text holds the value for ValueString and ValueMultiline.
	Text string
	// Items holds the entries for ValueList.
	Items []string
}

// String returns the value as text: the text for scalar kinds and the items
// joined by newlines for lists.
func (v Value) String() string {
	if v.Kind == ValueList {
		return strings.Join(v.Items, "\n")
	}
	return v.Text
}

// IsZero reports whether the value carries no content.
func (v Value) IsZero() bool {
	if v.Kind == ValueList {
		return len(v.Items) == 0
	}
	return v.Text == ""
}

// Annotation is a decorator attached to the pair or block that follows it, such
// as @description('Page size') or @number.
type Annotation struct {
	Name string
	// Arg is the decoded argument; meaningful only when HasArg is true.
	Arg string
	// HasArg reports whether the annotation was written with parentheses.
	HasArg bool
	Span   Span
}

// Pair is one key/value entry inside a dictionary block.
//
// A leading '~' in the source marks the pair disabled; the marker is stripped
// from Key and reported by Disabled. Duplicate keys are preserved as separate
// pairs in source order.
type Pair struct {
	Key         string
	Disabled    bool
	Annotations []Annotation
	Value       Value
	Span        Span
}

// DictBlock is a block whose body is a list of key/value pairs: meta, headers,
// an HTTP method block, auth:*, params:query, settings, and so on.
type DictBlock struct {
	Name  string
	Pairs []Pair
	Span  Span

	// AssertSyntax records that the block is an assert block, whose keys may
	// contain spaces. It is informational for consumers and printers.
	AssertSyntax bool
}

// BlockName implements Block.
func (b *DictBlock) BlockName() string { return b.Name }

// BlockSpan implements Block.
func (b *DictBlock) BlockSpan() Span { return b.Span }

// Get returns the value of the first enabled pair with the given key.
func (b *DictBlock) Get(key string) (Value, bool) {
	for _, p := range b.Pairs {
		if p.Key == key && !p.Disabled {
			return p.Value, true
		}
	}
	return Value{}, false
}

// GetString returns the text of the first enabled pair with the given key.
func (b *DictBlock) GetString(key string) (string, bool) {
	v, ok := b.Get(key)
	if !ok {
		return "", false
	}
	return v.String(), true
}

// TextBlock is a block whose body is raw text: body:*, script:*, tests, docs and
// example. The text is stored without the surrounding braces and without the
// newline that precedes the closing brace, and keeps the file's relative
// indentation so a save is byte faithful.
type TextBlock struct {
	Name string
	Text string
	Span Span
	// Example marks an example block, whose content has its own grammar.
	Example bool
}

// BlockName implements Block.
func (b *TextBlock) BlockName() string { return b.Name }

// BlockSpan implements Block.
func (b *TextBlock) BlockSpan() Span { return b.Span }

// RawBlock is a block the parser preserved verbatim because its name or shape is
// not known. Raw includes the header line, the body and the closing brace.
type RawBlock struct {
	Name string
	Raw  string
	Span Span
}

// BlockName implements Block.
func (b *RawBlock) BlockName() string { return b.Name }

// BlockSpan implements Block.
func (b *RawBlock) BlockSpan() Span { return b.Span }

// FindBlocks returns every block with the given name, in source order.
func (f *File) FindBlocks(name string) []Block {
	var out []Block
	for _, b := range f.Blocks {
		if b.BlockName() == name {
			out = append(out, b)
		}
	}
	return out
}

// FirstDict returns the first dictionary block with the given name.
func (f *File) FirstDict(name string) (*DictBlock, bool) {
	for _, b := range f.Blocks {
		if d, ok := b.(*DictBlock); ok && d.Name == name {
			return d, true
		}
	}
	return nil, false
}

// FirstText returns the first text block with the given name.
func (f *File) FirstText(name string) (*TextBlock, bool) {
	for _, b := range f.Blocks {
		if t, ok := b.(*TextBlock); ok && t.Name == name {
			return t, true
		}
	}
	return nil, false
}
