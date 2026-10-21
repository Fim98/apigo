// Package collection models Bruno collections: requests, folders, collection
// defaults, environments and configuration.
//
// The model is typed: no map[string]any carries request data. Parsing goes
// .bru -> bru.File -> model, and printing goes model -> bru.File -> .bru, so
// unknown blocks survive a load/save cycle and the file format stays the single
// source of truth.
package collection

import (
	"errors"
	"fmt"
	"strings"

	"apigo/internal/bru"
	"apigo/internal/opencollection"
)

// MaxFileSize is the size above which a .bru file is not parsed automatically.
// It matches Bruno's watcher cap of 2.5 MiB.
const MaxFileSize = 2560 * 1024

// ErrTooLarge is returned by the loader when a request file exceeds MaxFileSize.
var ErrTooLarge = errors.New("bru file exceeds the 2.5 MiB parse limit")

// RequestType is the protocol a request speaks.
type RequestType string

// Supported request types.
const (
	TypeHTTP    RequestType = "http"
	TypeGraphQL RequestType = "graphql"
	TypeGRPC    RequestType = "grpc"
	TypeWS      RequestType = "ws"
)

// Meta is the display metadata shared by requests and folders.
type Meta struct {
	Name string
	Type string
	Seq  int
	Tags []string
	// Description is the item-level description. `.bru` keeps it in a docs
	// block; OpenCollection has an `info.description` field beside it.
	Description string
	// Extra holds meta keys this version does not model, in source order.
	Extra []bru.Pair
}

// KeyValue is an ordered, possibly disabled name/value pair. Duplicates are
// preserved because HTTP allows them.
type KeyValue struct {
	Name        string
	Value       string
	Enabled     bool
	Description string
	Annotations []bru.Annotation
}

// Param is a query or path parameter.
type Param struct {
	KeyValue
	// Type is "query" or "path".
	Type string
}

// FormField is one entry of a form-urlencoded or multipart body.
type FormField struct {
	KeyValue
	// Type is "text" or "file".
	Type string
	// ContentType comes from a trailing @contentType(...) annotation.
	ContentType string
	// Files lists the file paths of a multipart file field.
	Files []string
}

// FileEntry is one entry of a body:file block.
type FileEntry struct {
	Path        string
	Selected    bool
	ContentType string
}

// Auth is the authentication configuration of a request, folder or collection.
//
// Mode is one of the Bruno mode names ("inherit", "none", "basic", "bearer",
// "digest", "ntlm", "oauth1", "oauth2", "apikey", "awsv4", "wsse",
// "akamai-edgegrid") or a custom mode. Params holds the mode block's fields.
type Auth struct {
	Mode   string
	Params []KeyValue
	// Additional holds OAuth2 additional-parameter blocks keyed by scope then
	// placement, e.g. Additional["access_token_req"]["body"].
	Additional map[string]map[string][]KeyValue
}

// BodyMode names the body variant in use.
type BodyMode string

// Body modes.
const (
	BodyNone           BodyMode = "none"
	BodyJSON           BodyMode = "json"
	BodyText           BodyMode = "text"
	BodyXML            BodyMode = "xml"
	BodySPARQL         BodyMode = "sparql"
	BodyGraphQL        BodyMode = "graphql"
	BodyFormURLEncoded BodyMode = "formUrlEncoded"
	BodyMultipart      BodyMode = "multipartForm"
	BodyFile           BodyMode = "file"
	BodyGRPC           BodyMode = "grpc"
	BodyWS             BodyMode = "ws"
)

// Body is a request body.
type Body struct {
	Mode BodyMode
	// Text is the raw content of json/text/xml/sparql/graphql bodies.
	Text string
	// GraphQLVars is the JSON object of a GraphQL variables block.
	GraphQLVars string
	// Form holds form-urlencoded and multipart entries.
	Form []FormField
	// Files holds body:file entries.
	Files []FileEntry
	// Pairs holds body:grpc and body:ws fields (name/content/type/selected).
	Pairs []KeyValue
}

// VarKind is the declared type of a variable.
type VarKind string

// Variable data types.
const (
	VarString  VarKind = "string"
	VarNumber  VarKind = "number"
	VarBoolean VarKind = "boolean"
	VarObject  VarKind = "object"
)

// Variable is a collection, folder, request or environment variable.
type Variable struct {
	Name        string
	Value       string
	Enabled     bool
	Kind        VarKind
	Local       bool
	Secret      bool
	Description string
	Annotations []bru.Annotation
}

// Assertion is one line of an assert block.
type Assertion struct {
	Expression  string
	Operator    string
	Operand     string
	Enabled     bool
	Description string
}

// Scripts holds the request lifecycle scripts.
type Scripts struct {
	PreRequest   string
	PostResponse string
	// GRPC maps a hook name (before-call-start, before-message-send,
	// after-message-receive, after-call-end) to its source.
	GRPC map[string]string
}

// IsZero reports whether no script is set.
func (s Scripts) IsZero() bool {
	return s.PreRequest == "" && s.PostResponse == "" && len(s.GRPC) == 0
}

// Settings are the per-request transport options.
//
// Pointer fields distinguish "unset" from "set to the zero value": a missing
// setting means Bruno's default, not Go's zero value.
type Settings struct {
	EncodeURL                  bool
	FollowRedirects            *bool
	MaxRedirects               *int
	TimeoutMS                  int
	TimeoutInherit             bool
	ForwardAuthorizationHeader *bool
	OmitHeaders                []string
	KeepAliveInterval          *int
	// Extra holds settings this version does not model.
	Extra []bru.Pair
}

// Example is a saved request/response example. Content is the example block's
// body, preserved verbatim until the example grammar is modelled.
type Example struct {
	Name    string
	Content string
}

// Request is a single request file.
type Request struct {
	// Path is the absolute file path and RelPath the path relative to the
	// collection root, always slash separated.
	Path    string
	RelPath string

	Meta Meta

	Method   string
	URL      string
	BodyMode string
	AuthMode string

	Params   []Param
	Headers  []KeyValue
	Metadata []KeyValue

	Auth *Auth
	Body Body

	VarsPre  []Variable
	VarsPost []Variable

	Assertions []Assertion
	Scripts    Scripts
	Tests      string
	Docs       string
	Settings   Settings
	Examples   []Example

	// Unknown holds blocks the model does not understand, in source order.
	Unknown []bru.Block

	// Source is the parsed document a YAML request was read from, kept so a
	// writer can preserve fields the model does not cover. It is nil for `.bru`.
	Source *opencollection.Document
}

// Name returns the request's display name.
func (r *Request) Name() string {
	if r.Meta.Name != "" {
		return r.Meta.Name
	}
	base := r.RelPath
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".bru")
}

// Folder is a collection folder with its inherited defaults.
type Folder struct {
	Path    string
	RelPath string

	Meta Meta

	Headers  []KeyValue
	Auth     *Auth
	VarsPre  []Variable
	VarsPost []Variable
	Scripts  Scripts
	Tests    string
	Docs     string
	Settings Settings

	// Source is the parsed document a YAML folder was read from; nil for `.bru`.
	Source *opencollection.Document
}

// Node is one entry in the collection tree: either a folder or a request.
type Node struct {
	Name    string
	Path    string
	RelPath string
	// Folder is set for directory nodes, Request for request files.
	Folder  *Folder
	Request *Request
	// Children are a folder's entries in display order.
	Children []*Node
	Parent   *Node
}

// IsFolder reports whether the node is a folder.
func (n *Node) IsFolder() bool { return n.Folder != nil }

// Walk visits the node and its descendants in display order.
func (n *Node) Walk(fn func(*Node) error) error {
	if err := fn(n); err != nil {
		return err
	}
	for _, child := range n.Children {
		if err := child.Walk(fn); err != nil {
			return err
		}
	}
	return nil
}

// Chain returns the node's ancestors from the collection root to the node.
func (n *Node) Chain() []*Node {
	var chain []*Node
	for cur := n; cur != nil; cur = cur.Parent {
		chain = append(chain, cur)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// Collection is a loaded collection.
type Collection struct {
	Root string
	// Format records how the collection is stored: `.bru` files or
	// OpenCollection YAML.
	Format Format
	// Config is bruno.json.
	Config Config
	// Defaults holds collection.bru: the headers, auth, vars and scripts every
	// request inherits.
	Defaults *Folder
	// Environments are the files in environments/, in name order.
	Environments []*Environment
	// Tree is the root's entries in display order.
	Tree []*Node
	// root is the synthetic node that carries collection.bru, so a node's
	// ancestor chain includes the collection defaults.
	root *Node
	// Warnings are non-fatal problems found while loading (a broken file, an
	// oversized file); the rest of the collection still loads.
	Warnings []error
}

// RootNode returns the synthetic node holding collection.bru.
func (c *Collection) RootNode() *Node { return c.root }

// Walk visits every node in the tree.
func (c *Collection) Walk(fn func(*Node) error) error {
	for _, node := range c.Tree {
		if err := node.Walk(fn); err != nil {
			return err
		}
	}
	return nil
}

// Requests returns every request in the collection in tree order.
func (c *Collection) Requests() []*Request {
	var out []*Request
	_ = c.Walk(func(n *Node) error {
		if n.Request != nil {
			out = append(out, n.Request)
		}
		return nil
	})
	return out
}

// ResolvedHeaders merges headers along a node's chain, the collection first.
// A later value for the same header name replaces the earlier one and keeps its
// original position.
func ResolvedHeaders(n *Node) []KeyValue {
	var out []KeyValue
	index := map[string]int{}

	add := func(pairs []KeyValue) {
		for _, kv := range pairs {
			if i, ok := index[strings.ToLower(kv.Name)]; ok {
				out[i] = kv
				continue
			}
			index[strings.ToLower(kv.Name)] = len(out)
			out = append(out, kv)
		}
	}

	var reqHeaders []KeyValue
	if n.Request != nil {
		reqHeaders = n.Request.Headers
	}
	for _, link := range n.Chain() {
		if link.Folder != nil {
			add(link.Folder.Headers)
		}
	}
	add(reqHeaders)
	return out
}

// ResolvedAuth returns the auth that applies to a node: the nearest
// non-inherit mode wins, request before folder before collection.
func ResolvedAuth(n *Node) *Auth {
	if n.Request != nil && n.Request.Auth != nil {
		switch n.Request.Auth.Mode {
		case "none":
			return &Auth{Mode: "none"}
		case "":
		case "inherit":
		default:
			return n.Request.Auth
		}
	}
	chain := n.Chain()
	for i := len(chain) - 1; i >= 0; i-- {
		link := chain[i]
		if link.Folder == nil || link.Folder.Auth == nil {
			continue
		}
		switch link.Folder.Auth.Mode {
		case "":
			continue
		case "inherit":
			continue
		case "none":
			return &Auth{Mode: "none"}
		default:
			return link.Folder.Auth
		}
	}
	return nil
}

// ResolvedVarsPre merges pre-request variables along the chain.
func ResolvedVarsPre(n *Node) []Variable {
	var out []Variable
	for _, link := range n.Chain() {
		if link.Folder != nil {
			out = append(out, link.Folder.VarsPre...)
		}
	}
	if n.Request != nil {
		out = append(out, n.Request.VarsPre...)
	}
	return out
}

// ResolvedVarsPost merges post-response variables along the chain.
func ResolvedVarsPost(n *Node) []Variable {
	var out []Variable
	for _, link := range n.Chain() {
		if link.Folder != nil {
			out = append(out, link.Folder.VarsPost...)
		}
	}
	if n.Request != nil {
		out = append(out, n.Request.VarsPost...)
	}
	return out
}

// ResolvedScripts concatenates lifecycle scripts along the chain, outermost
// first, the way Bruno runs inherited scripts.
func ResolvedScripts(n *Node) Scripts {
	var out Scripts
	for _, link := range n.Chain() {
		if link.Folder == nil {
			continue
		}
		out.PreRequest = joinScript(out.PreRequest, link.Folder.Scripts.PreRequest)
		out.PostResponse = joinScript(out.PostResponse, link.Folder.Scripts.PostResponse)
		for hook, src := range link.Folder.Scripts.GRPC {
			if out.GRPC == nil {
				out.GRPC = map[string]string{}
			}
			out.GRPC[hook] = joinScript(out.GRPC[hook], src)
		}
	}
	if n.Request != nil {
		out.PreRequest = joinScript(out.PreRequest, n.Request.Scripts.PreRequest)
		out.PostResponse = joinScript(out.PostResponse, n.Request.Scripts.PostResponse)
		for hook, src := range n.Request.Scripts.GRPC {
			if out.GRPC == nil {
				out.GRPC = map[string]string{}
			}
			out.GRPC[hook] = joinScript(out.GRPC[hook], src)
		}
	}
	return out
}

func joinScript(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "\n" + b
	}
}

// ErrNotFound is returned when a collection path does not exist.
var ErrNotFound = errors.New("collection not found")

// ConfigError reports a problem with bruno.json.
type ConfigError struct {
	Path string
	Err  error
}

// Error implements error.
func (e *ConfigError) Error() string { return fmt.Sprintf("%s: %v", e.Path, e.Err) }

// Unwrap returns the underlying error.
func (e *ConfigError) Unwrap() error { return e.Err }

// Outdent removes two leading spaces from every line that has them. Text
// blocks keep the file's relative indentation so a save is byte faithful;
// display and script execution call Outdent first.
func Outdent(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "  ")
	}
	return strings.Join(lines, "\n")
}
