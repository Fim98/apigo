// Package opencollection reads OpenCollection YAML, the collection format Bruno
// 2.x creates by default.
//
// The package is a leaf: it knows about YAML documents, not about apigo's
// collection model. internal/collection maps the document onto the model, so the
// YAML dependency stays behind this seam. See docs/OPENCOLLECTION.md and
// docs/decisions/ADR-0010-opencollection-yaml.md.
package opencollection

import "gopkg.in/yaml.v3"

// Kind is the shape of a node.
type Kind uint8

// Node kinds.
const (
	// Invalid is a missing node.
	Invalid Kind = iota
	// Map is a mapping with string keys.
	Map
	// Sequence is a list.
	Sequence
	// Scalar is a string, number, boolean or null.
	Scalar
)

// Node is a read-only view over a YAML node. A nil *Node is valid and behaves
// like a missing value, so mapping code never has to check for nil twice.
type Node struct {
	n *yaml.Node
}

func wrap(n *yaml.Node) *Node {
	if n == nil {
		return nil
	}
	return &Node{n: n}
}

// resolve follows aliases so callers never see an AliasNode.
func (n *Node) resolve() *Node {
	if n == nil || n.n == nil {
		return nil
	}
	if n.n.Kind == yaml.AliasNode && n.n.Alias != nil {
		return wrap(n.n.Alias)
	}
	return n
}

// Valid reports whether the node exists.
func (n *Node) Valid() bool { return n != nil && n.n != nil }

// Kind returns the node's shape.
func (n *Node) Kind() Kind {
	n = n.resolve()
	if n == nil {
		return Invalid
	}
	switch n.n.Kind {
	case yaml.MappingNode:
		return Map
	case yaml.SequenceNode:
		return Sequence
	case yaml.ScalarNode:
		return Scalar
	default:
		return Invalid
	}
}

// Line and Column locate the node in the source, for error messages.
func (n *Node) Line() int {
	if !n.Valid() {
		return 0
	}
	return n.n.Line
}

// Column returns the node's one-based column.
func (n *Node) Column() int {
	if !n.Valid() {
		return 0
	}
	return n.n.Column
}

// IsMap reports whether the node is a mapping.
func (n *Node) IsMap() bool { return n.Kind() == Map }

// IsSequence reports whether the node is a list.
func (n *Node) IsSequence() bool { return n.Kind() == Sequence }

// IsScalar reports whether the node is a scalar value.
func (n *Node) IsScalar() bool { return n.Kind() == Scalar }

// IsNull reports whether the node is an explicit or implicit null.
func (n *Node) IsNull() bool {
	n = n.resolve()
	if n == nil {
		return true
	}
	return n.n.Kind == yaml.ScalarNode && n.n.Tag == "!!null"
}

// Get returns the value for key in a mapping, or nil.
func (n *Node) Get(key string) *Node {
	n = n.resolve()
	if n == nil || n.n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.n.Content); i += 2 {
		if n.n.Content[i].Value == key {
			return wrap(n.n.Content[i+1])
		}
	}
	return nil
}

// Has reports whether a mapping contains key.
func (n *Node) Has(key string) bool { return n.Get(key) != nil }

// Each visits the entries of a mapping in source order, or the items of a
// sequence with an empty key. Returning false stops the walk.
func (n *Node) Each(fn func(key string, value *Node) bool) {
	n = n.resolve()
	if n == nil {
		return
	}
	switch n.n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.n.Content); i += 2 {
			if !fn(n.n.Content[i].Value, wrap(n.n.Content[i+1])) {
				return
			}
		}
	case yaml.SequenceNode:
		for _, item := range n.n.Content {
			if !fn("", wrap(item)) {
				return
			}
		}
	}
}

// Len returns the number of mapping entries or sequence items.
func (n *Node) Len() int {
	n = n.resolve()
	if n == nil {
		return 0
	}
	switch n.n.Kind {
	case yaml.MappingNode:
		return len(n.n.Content) / 2
	case yaml.SequenceNode:
		return len(n.n.Content)
	default:
		return 0
	}
}

// Items returns a sequence's items, or nil.
func (n *Node) Items() []*Node {
	n = n.resolve()
	if n == nil || n.n.Kind != yaml.SequenceNode {
		return nil
	}
	items := make([]*Node, 0, len(n.n.Content))
	for _, item := range n.n.Content {
		items = append(items, wrap(item))
	}
	return items
}

// String returns the scalar's text. Non-scalars and nulls return "".
func (n *Node) String() string {
	n = n.resolve()
	if n == nil || n.n.Kind != yaml.ScalarNode || n.n.Tag == "!!null" {
		return ""
	}
	return n.n.Value
}

// StringOr returns the scalar's text, or def when the node is missing or null.
func (n *Node) StringOr(def string) string {
	if n.IsNull() {
		return def
	}
	return n.String()
}

// Bool decodes a scalar boolean and reports whether it was one.
func (n *Node) Bool() (bool, bool) {
	n = n.resolve()
	if n == nil || n.n.Kind != yaml.ScalarNode {
		return false, false
	}
	var v bool
	if err := n.n.Decode(&v); err != nil {
		return false, false
	}
	return v, true
}

// Int decodes a scalar integer and reports whether it was one.
func (n *Node) Int() (int, bool) {
	n = n.resolve()
	if n == nil || n.n.Kind != yaml.ScalarNode {
		return 0, false
	}
	var v int
	if err := n.n.Decode(&v); err != nil {
		return 0, false
	}
	return v, true
}

// Strings returns a sequence of scalars as strings.
func (n *Node) Strings() []string {
	items := n.Items()
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsScalar() {
			out = append(out, item.String())
		}
	}
	return out
}
