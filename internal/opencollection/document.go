package opencollection

import (
	"bytes"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Document is a parsed OpenCollection YAML file.
//
// It keeps the whole node tree, so unknown keys, key order and comments survive
// a read. The writer patches this tree rather than re-serialising a map, which is
// what keeps fields apigo does not model yet byte-identical.
type Document struct {
	src  []byte
	root *Node
}

// Error is a YAML parse error with a source position.
type Error struct {
	Line   int
	Column int
	Msg    string
}

// Error implements error.
func (e *Error) Error() string {
	loc := ""
	if e.Line > 0 {
		loc = "line " + strconv.Itoa(e.Line)
		if e.Column > 0 {
			loc += ":" + strconv.Itoa(e.Column)
		}
		loc += ": "
	}
	return "opencollection: " + loc + e.Msg
}

// Parse decodes a YAML document. An empty document yields a valid Document whose
// root is absent, which callers treat as "no fields".
func Parse(src []byte) (*Document, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(src, &node); err != nil {
		return nil, yamlError(err)
	}
	doc := &Document{src: src}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) > 0 {
			doc.root = wrap(node.Content[0])
		}
	} else if node.Kind != 0 {
		doc.root = wrap(&node)
	}
	return doc, nil
}

// Root returns the document's top-level node.
func (d *Document) Root() *Node {
	if d == nil {
		return nil
	}
	return d.root
}

// Source returns the bytes the document was parsed from.
func (d *Document) Source() []byte {
	if d == nil {
		return nil
	}
	return d.src
}

// Bytes re-encodes the document with the canonical style: two-space indent and
// LF line endings. Unknown fields survive because the whole node tree is
// marshalled, not a decoded map.
func (d *Document) Bytes() ([]byte, error) {
	if d == nil || !d.root.Valid() {
		return []byte{}, nil
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(d.root.n); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(buf.Bytes(), []byte("\r\n"), []byte("\n")), nil
}

// Clone deep-copies the document so callers can patch it safely.
func (d *Document) Clone() *Document {
	if d == nil {
		return &Document{}
	}
	return &Document{src: d.src, root: d.root.Clone()}
}

// yamlError converts a yaml.v3 error into one with a position. yaml.v3 only
// reports "yaml: line N: message", so the position is parsed out of the text.
func yamlError(err error) error {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "yaml: ")
	line, column := 0, 0
	if rest, ok := strings.CutPrefix(msg, "line "); ok {
		num, tail, found := strings.Cut(rest, ":")
		if found {
			if n, convErr := strconv.Atoi(num); convErr == nil {
				line = n
			}
			msg = strings.TrimSpace(tail)
		}
	}
	return &Error{Line: line, Column: column, Msg: msg}
}

// NewDocument wraps a node as a standalone document. It is how fragments, for
// example one example block, are rendered back to YAML.
func NewDocument(n *Node) *Document {
	if n == nil || !n.Valid() {
		return &Document{}
	}
	return &Document{root: n}
}
