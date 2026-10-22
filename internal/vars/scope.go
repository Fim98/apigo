// Package vars resolves the variable scopes of a request.
//
// A request sees eight layers. From lowest to highest priority:
//
//	global environment < collection < environment < folder < request <
//	oauth2 credentials < runtime < prompt
//
// Within the folder layer the nearest folder wins over its ancestors; within
// every other layer the later value wins.
//
// Two namespaces sit outside that order and are addressed explicitly:
// `process.env.<NAME>` (the OS environment merged with the collection `.env`)
// and `$oauth2.<credentialsId>.<field>` (credential values). They are never
// merged into bare names, so `{{TOKEN}}` does not pick up `.env`'s TOKEN.
package vars

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"apigo/internal/collection"
)

// Scope is one layer of variables.
type Scope uint8

// The layers, in the order they are declared rather than the order they win.
const (
	// ScopeGlobal is the workspace-scoped global environment.
	ScopeGlobal Scope = iota + 1
	// ScopeCollection is collection.bru's vars:pre-request.
	ScopeCollection
	// ScopeEnvironment is the selected environment after extends and overrides.
	ScopeEnvironment
	// ScopeFolder is the merged folder chain, nearest folder last.
	ScopeFolder
	// ScopeRequest is the request's own vars:pre-request.
	ScopeRequest
	// ScopeOAuth2 holds $oauth2.<credentialsId>.<field> credentials.
	ScopeOAuth2
	// ScopeRuntime holds variables set by scripts during the run.
	ScopeRuntime
	// ScopePrompt holds answers to {{?Prompt}} questions.
	ScopePrompt
	// ScopeProcessEnv marks a value read from the process.env namespace. It is
	// not a layer: it never takes part in the precedence order.
	ScopeProcessEnv
)

// precedence lists the layers from lowest to highest priority, which is the
// order REQUEST_ENGINE.md §2 documents.
var precedence = [...]Scope{
	ScopeGlobal,
	ScopeCollection,
	ScopeEnvironment,
	ScopeFolder,
	ScopeRequest,
	ScopeOAuth2,
	ScopeRuntime,
	ScopePrompt,
}

// isLayer reports whether scope is one of the eight precedence layers.
func isLayer(scope Scope) bool {
	for _, layer := range precedence {
		if layer == scope {
			return true
		}
	}
	return false
}

// String returns the layer name used by the UI and the CLI.
func (s Scope) String() string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeCollection:
		return "collection"
	case ScopeEnvironment:
		return "environment"
	case ScopeFolder:
		return "folder"
	case ScopeRequest:
		return "request"
	case ScopeOAuth2:
		return "oauth2"
	case ScopeRuntime:
		return "runtime"
	case ScopePrompt:
		return "prompt"
	case ScopeProcessEnv:
		return "process.env"
	default:
		return "unknown"
	}
}

// Value is one resolved variable.
type Value struct {
	// Text is the variable's value as text. Object values hold JSON.
	Text string
	// Kind is the declared type, which a whole-string reference keeps.
	Kind collection.VarKind
	// Scope is the layer the value came from.
	Scope Scope
	// Secret marks a value that must be masked in the UI and logs.
	Secret bool
}

// String returns the value's text.
func (v Value) String() string { return v.Text }

// Set is the variable set of one request.
//
// It is not safe for concurrent use: the engine builds one per request and the
// script runtime mutates it between the pre-request script and the send.
type Set struct {
	layers     map[Scope]map[string]Value
	processEnv map[string]string
}

// NewSet returns an empty set.
func NewSet() *Set {
	return &Set{layers: make(map[Scope]map[string]Value, len(precedence))}
}

// Precedence returns the layers from lowest to highest priority.
func Precedence() []Scope {
	out := make([]Scope, len(precedence))
	copy(out, precedence[:])
	return out
}

// SetProcessEnv sets the process.env namespace. The map is copied, so the
// caller can reuse or mutate its own map.
func (s *Set) SetProcessEnv(env map[string]string) {
	if len(env) == 0 {
		s.processEnv = nil
		return
	}
	s.processEnv = make(map[string]string, len(env))
	for name, value := range env {
		s.processEnv[name] = value
	}
}

// Load adds variables to a layer. Disabled variables and names that are empty
// are skipped, so a disabled request variable lets a lower layer show through.
//
// A Variable built by hand rather than parsed from a file must set Enabled:
// true; the zero value means disabled.
func (s *Set) Load(scope Scope, variables []collection.Variable) {
	for _, v := range variables {
		if v.Name == "" || !v.Enabled {
			continue
		}
		s.Put(scope, v)
	}
}

// Put records one variable in a layer, replacing an earlier value of the same
// name. Unlike Load it ignores the Enabled flag: it is for values the engine
// decides to set, such as a runtime variable or a prompt answer.
//
// Put panics when scope is not one of the eight layers. ScopeProcessEnv is a
// read-only namespace, so writing it would create a layer that no lookup reads.
func (s *Set) Put(scope Scope, v collection.Variable) {
	if !isLayer(scope) {
		panic("vars: Put with non-layer scope " + scope.String())
	}
	if v.Name == "" {
		return
	}
	if s.layers == nil {
		s.layers = make(map[Scope]map[string]Value, len(precedence))
	}
	layer := s.layers[scope]
	if layer == nil {
		layer = make(map[string]Value, 4)
		s.layers[scope] = layer
	}
	kind := v.Kind
	if kind == "" {
		kind = collection.VarString
	}
	layer[v.Name] = Value{Text: v.Value, Kind: kind, Scope: scope, Secret: v.Secret}
}

// PutText records a string variable in a layer.
func (s *Set) PutText(scope Scope, name, text string) {
	s.Put(scope, collection.Variable{Name: name, Value: text, Kind: collection.VarString})
}

// Prefixed namespaces that are not layers.
const (
	processEnvPrefix = "process.env."
	oauth2Prefix     = "$oauth2."
)

// Lookup resolves a name.
//
// An exact name wins over path traversal, which is what makes a variable whose
// name contains dots usable and lets `$oauth2.<id>.<field>` resolve. Otherwise
// the name is a path: `user.address.city`, `items[0]`, `items[0].name`, where
// the first segment is a variable holding a JSON object (a typed `@object`
// variable or a runtime object).
//
// `process.env.NAME` reads the process namespace. An unknown name reports false
// and leaves `{{name}}` in place at the call site.
func (s *Set) Lookup(name string) (Value, bool) {
	if name == "" {
		return Value{}, false
	}
	if v, ok := s.exact(name); ok {
		return v, true
	}
	if strings.HasPrefix(name, processEnvPrefix) {
		value, ok := s.processEnv[strings.TrimPrefix(name, processEnvPrefix)]
		if !ok {
			return Value{}, false
		}
		return Value{Text: value, Kind: collection.VarString, Scope: ScopeProcessEnv}, true
	}

	segments, ok := parsePath(name)
	if !ok || len(segments) < 2 || segments[0].isIndex {
		return Value{}, false
	}
	head, ok := s.exact(segments[0].key)
	if !ok {
		return Value{}, false
	}
	return walk(head, segments[1:])
}

// ScopeOf reports the layer a name resolves in, including the process.env
// pseudo-layer.
func (s *Set) ScopeOf(name string) (Scope, bool) {
	v, ok := s.Lookup(name)
	if !ok {
		return 0, false
	}
	return v.Scope, true
}

// Value returns the value recorded in one layer, ignoring precedence. It is how
// a variable viewer shows a shadowed entry alongside the winning one.
func (s *Set) Value(scope Scope, name string) (Value, bool) {
	v, ok := s.layers[scope][name]
	return v, ok
}

// Names returns the names in one layer, sorted.
func (s *Set) Names(scope Scope) []string {
	layer := s.layers[scope]
	names := make([]string, 0, len(layer))
	for name := range layer {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// exact searches every layer from the highest priority down.
func (s *Set) exact(name string) (Value, bool) {
	for i := len(precedence) - 1; i >= 0; i-- {
		if layer := s.layers[precedence[i]]; layer != nil {
			if v, ok := layer[name]; ok {
				return v, true
			}
		}
	}
	return Value{}, false
}

// segment is one step of a path: a key or an array index.
type segment struct {
	key     string
	index   int
	isIndex bool
}

// parsePath splits `user.address.city` or `items[0].name` into segments. A
// trailing or leading dot, an empty key or an unbalanced bracket is rejected.
func parsePath(path string) ([]segment, bool) {
	var segments []segment
	var key strings.Builder

	flushKey := func() bool {
		if key.Len() == 0 {
			return false
		}
		segments = append(segments, segment{key: key.String()})
		key.Reset()
		return true
	}

	for i := 0; i < len(path); i++ {
		switch c := path[i]; c {
		case '.':
			// A dot is allowed straight after an index (`items[0].name`).
			if key.Len() == 0 && len(segments) > 0 && segments[len(segments)-1].isIndex {
				continue
			}
			if !flushKey() {
				return nil, false
			}
		case '[':
			if key.Len() > 0 {
				flushKey()
			}
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, false
			}
			n, err := strconv.Atoi(path[i+1 : i+end])
			if err != nil || n < 0 {
				return nil, false
			}
			segments = append(segments, segment{index: n, isIndex: true})
			i += end
			if i+1 < len(path) && path[i+1] != '.' && path[i+1] != '[' {
				// `items[0]name` is not a path.
				return nil, false
			}
		default:
			key.WriteByte(c)
		}
	}
	if key.Len() > 0 {
		flushKey()
	}
	if len(segments) == 0 {
		return nil, false
	}
	return segments, true
}

// walk follows a path into a JSON object or array value. The first segment is
// the variable itself; `decodeJSON` keeps member order and number spelling so an
// interpolated object looks the way it was written.
func walk(head Value, segments []segment) (Value, bool) {
	if head.Kind != collection.VarObject {
		return Value{}, false
	}
	doc, err := decodeJSON(head.Text)
	if err != nil {
		return Value{}, false
	}

	current := doc
	for _, seg := range segments {
		switch current.kind {
		case jsonObject:
			if seg.isIndex {
				return Value{}, false
			}
			found := false
			for _, member := range current.obj {
				if member.key == seg.key {
					current, found = member.value, true
					break
				}
			}
			if !found {
				return Value{}, false
			}
		case jsonArray:
			index, ok := seg.arrayIndex()
			if !ok || index >= len(current.arr) {
				return Value{}, false
			}
			current = current.arr[index]
		default:
			return Value{}, false
		}
	}

	kind, text := current.value()
	return Value{Text: text, Kind: kind, Scope: head.Scope, Secret: head.Secret}, true
}

// arrayIndex reports the index a segment selects. A dotted numeric key is
// accepted too, because lodash `get` treats `tags.0` and `tags[0]` the same.
func (seg segment) arrayIndex() (int, bool) {
	if seg.isIndex {
		return seg.index, true
	}
	if seg.key == "" {
		return 0, false
	}
	for i := 0; i < len(seg.key); i++ {
		if seg.key[i] < '0' || seg.key[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(seg.key)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// Options are the inputs of a request's variable set that do not live in the
// collection tree.
type Options struct {
	// Environment is the selected environment. Extends is resolved through the
	// collection's other environments; nil means no environment layer.
	Environment *collection.Environment
	// EnvironmentOverrides are `--env-var` values, applied above the environment
	// file.
	EnvironmentOverrides map[string]string
	// Global is the workspace-scoped global environment, below collection
	// variables.
	Global []collection.Variable
	// ProcessEnv is the merged process.env namespace, usually
	// Collection.ProcessEnv(os.Environ(), workspaceDotEnv).
	ProcessEnv map[string]string
	// OAuth2 holds credentials keyed `$oauth2.<credentialsId>.<field>`.
	OAuth2 []collection.Variable
	// Runtime holds variables already set during the run, for example by an
	// earlier request's script.
	Runtime []collection.Variable
	// Prompt holds answers to `{{?Prompt}}` questions, keyed by prompt name.
	Prompt map[string]string
}

// ForRequest builds the variable set of one request or folder node.
//
// Folder variables are collected from the root down, so the nearest folder wins
// over its ancestors. Collection variables come from c.Defaults (collection.bru
// or the YAML collection's `request.variables`), never from the node chain,
// which would otherwise count them twice.
func ForRequest(c *collection.Collection, node *collection.Node, opts Options) (*Set, error) {
	if c == nil {
		return nil, errors.New("vars: nil collection")
	}
	if node == nil {
		return nil, errors.New("vars: nil node")
	}

	set := NewSet()
	set.SetProcessEnv(opts.ProcessEnv)
	set.Load(ScopeGlobal, opts.Global)
	if c.Defaults != nil {
		set.Load(ScopeCollection, c.Defaults.VarsPre)
	}

	if opts.Environment != nil {
		resolved, err := c.ResolveEnvironment(opts.Environment)
		if err != nil {
			return nil, fmt.Errorf("vars: environment %q: %w", opts.Environment.Name, err)
		}
		set.Load(ScopeEnvironment, resolved)
	}
	for name, value := range opts.EnvironmentOverrides {
		set.PutText(ScopeEnvironment, name, value)
	}

	for _, link := range node.Chain() {
		if link.Folder == nil || link.Folder == c.Defaults {
			continue
		}
		set.Load(ScopeFolder, link.Folder.VarsPre)
	}
	if node.Request != nil {
		set.Load(ScopeRequest, node.Request.VarsPre)
	}

	set.Load(ScopeOAuth2, opts.OAuth2)
	set.Load(ScopeRuntime, opts.Runtime)
	for name, value := range opts.Prompt {
		set.PutText(ScopePrompt, name, value)
	}
	return set, nil
}
