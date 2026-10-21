package collection

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"apigo/internal/opencollection"
)

// OpenCollection file names.
const (
	// OpenCollectionFile marks a YAML collection and holds its settings.
	OpenCollectionFile = "opencollection.yml"
	// OpenCollectionAltFile is the accepted alternative spelling.
	OpenCollectionAltFile = "opencollection.yaml"
	// YAMLFolderFile holds a folder's defaults.
	YAMLFolderFile = "folder.yml"
	// YAMLFolderAltFile is the accepted alternative spelling.
	YAMLFolderAltFile = "folder.yaml"
)

// Format identifies how a collection is stored on disk.
type Format uint8

// Collection formats.
const (
	// FormatBRU is `.bru` files with bruno.json.
	FormatBRU Format = iota
	// FormatYAML is OpenCollection YAML.
	FormatYAML
)

// String returns the format name.
func (f Format) String() string {
	if f == FormatYAML {
		return "yaml"
	}
	return "bru"
}

// DetectFormat reports how a collection directory is stored.
//
// OpenCollection wins when both markers exist, matching Bruno 2.x; the caller is
// expected to warn the user, because the `.bru` files become invisible.
func DetectFormat(dir string) Format {
	for _, name := range []string{OpenCollectionFile, OpenCollectionAltFile} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return FormatYAML
		}
	}
	return FormatBRU
}

// openCollectionPath returns the collection file that exists in dir.
func openCollectionPath(dir string) (string, bool) {
	for _, name := range []string{OpenCollectionFile, OpenCollectionAltFile} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// yamlFolderPath returns a folder's settings file if it has one.
func yamlFolderPath(dir string) (string, bool) {
	for _, name := range []string{YAMLFolderFile, YAMLFolderAltFile} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// ParseYAMLCollection reads opencollection.yml into the collection config and the
// request defaults every request inherits.
func ParseYAMLCollection(path string, src []byte) (Config, *Folder, error) {
	doc, err := opencollection.Parse(src)
	if err != nil {
		return Config{}, nil, yamlPathError(path, err)
	}
	root := doc.Root()
	cfg := Config{
		Name:    root.Get("info").Get("name").String(),
		Type:    "collection",
		Version: root.Get("info").Get("version").String(),
	}
	defaults := &Folder{
		Path: path,
		Meta: Meta{Name: cfg.Name, Type: string(TypeHTTP), Seq: 1},
	}

	if defaultsNode := root.Get("request"); defaultsNode.Valid() {
		readFolderRequest(defaults, defaultsNode)
	}
	if docs := root.Get("docs"); docs.Valid() {
		defaults.Docs = docs.Get("content").String()
	}
	return cfg, defaults, nil
}

// ParseYAMLRequest reads one item file into the request model.
func ParseYAMLRequest(path, relPath string, src []byte) (*Request, error) {
	doc, err := opencollection.Parse(src)
	if err != nil {
		return nil, yamlPathError(path, err)
	}
	root := doc.Root()
	info := root.Get("info")

	req := &Request{Path: path, RelPath: relPath}
	req.Meta = yamlMeta(info)
	req.URL = ""
	req.Method = ""

	protocol := yamlProtocolNode(root, req.Meta.Type)
	if protocol.Valid() {
		req.Method = strings.ToUpper(protocol.Get("method").String())
		req.URL = protocol.Get("url").String()
		req.Headers = keyValuesFromYAML(protocol.Get("headers"))
		req.Params = paramsFromYAML(protocol.Get("params"))
		req.Metadata = keyValuesFromYAML(protocol.Get("metadata"))
		req.Auth = authFromYAML(protocol.Get("auth"))
		if req.Auth != nil {
			req.AuthMode = req.Auth.Mode
		}
		applyYAMLBody(&req.Body, protocol.Get("body"))
	}

	if runtime := root.Get("runtime"); runtime.Valid() {
		req.VarsPre = append(req.VarsPre, variablesFromYAML(runtime.Get("variables"))...)
		req.Assertions = append(req.Assertions, assertionsFromYAML(runtime.Get("assertions"))...)
		req.VarsPost = append(req.VarsPost, postResponseVarsFromYAML(runtime.Get("actions"))...)
		applyYAMLScripts(req, runtime.Get("scripts"))
	}

	if settings := root.Get("settings"); settings.Valid() {
		req.Settings = settingsFromYAML(settings)
	}
	if docs := root.Get("docs"); docs.Valid() {
		req.Docs = docs.Get("content").String()
	}
	for _, node := range root.Get("examples").Items() {
		req.Examples = append(req.Examples, Example{
			Name:    node.Get("name").String(),
			Content: yamlFragment(node),
		})
	}

	if req.Method == "" && req.Meta.Type == string(TypeHTTP) {
		req.Method = "GET"
	}
	if req.Body.Mode == "" {
		req.Body.Mode = BodyNone
	}
	return req, nil
}

// ParseYAMLFolder reads folder.yml into folder defaults.
func ParseYAMLFolder(path, relPath string, src []byte) (*Folder, error) {
	doc, err := opencollection.Parse(src)
	if err != nil {
		return nil, yamlPathError(path, err)
	}
	root := doc.Root()
	folder := &Folder{Path: path, RelPath: relPath}
	folder.Meta = yamlMeta(root.Get("info"))
	if folder.Meta.Type == "" || folder.Meta.Type == "folder" {
		folder.Meta.Type = string(TypeHTTP)
	}
	if requestNode := root.Get("request"); requestNode.Valid() {
		readFolderRequest(folder, requestNode)
	}
	if docs := root.Get("docs"); docs.Valid() {
		folder.Docs = docs.Get("content").String()
	}
	return folder, nil
}

// ParseYAMLEnvironment reads environments/<name>.yml.
func ParseYAMLEnvironment(path, name string, src []byte) (*Environment, error) {
	doc, err := opencollection.Parse(src)
	if err != nil {
		return nil, yamlPathError(path, err)
	}
	root := doc.Root()
	env := &Environment{Name: root.Get("name").StringOr(name), Path: path}
	if env.Name == "" {
		env.Name = name
	}
	env.Color = root.Get("color").String()
	env.Extends = yamlNames(root.Get("extends"))

	for _, node := range root.Get("variables").Items() {
		v := variableFromYAML(node)
		if secret, ok := node.Get("secret").Bool(); ok && secret {
			v.Secret = true
			v.Value = ""
			env.SecretNames = append(env.SecretNames, v.Name)
		}
		env.Vars = append(env.Vars, v)
	}

	if external := root.Get("externalSecrets"); external.Valid() {
		provider := ExternalSecretProvider{Provider: external.Get("type").String()}
		for _, node := range external.Get("variables").Items() {
			provider.Pairs = append(provider.Pairs, KeyValue{
				Name:    node.Get("name").String(),
				Value:   node.Get("value").String(),
				Enabled: !yamlDisabled(node),
			})
		}
		env.External = append(env.External, provider)
	}
	env.ApplySecrets()
	return env, nil
}

// yamlPathError adds the file path to a YAML error while keeping its position.
func yamlPathError(path string, err error) error {
	var ocErr *opencollection.Error
	if errors.As(err, &ocErr) {
		return fmt.Errorf("%s:%d: %w", path, ocErr.Line, err)
	}
	return fmt.Errorf("%s: %w", path, err)
}

// yamlMeta maps the info block onto Meta.
func yamlMeta(info *opencollection.Node) Meta {
	meta := Meta{
		Name: info.Get("name").String(),
		Type: info.Get("type").String(),
		Tags: info.Get("tags").Strings(),
	}
	if seq, ok := info.Get("seq").Int(); ok {
		meta.Seq = seq
	}
	if meta.Seq == 0 {
		meta.Seq = 1
	}
	if meta.Type == "" {
		meta.Type = string(TypeHTTP)
	}
	return meta
}

// yamlProtocolNode returns the block that carries the request fields: http,
// graphql, grpc or websocket.
func yamlProtocolNode(root *opencollection.Node, itemType string) *opencollection.Node {
	switch itemType {
	case "graphql", "grpc", "websocket", "ws":
		if node := root.Get(itemType); node.Valid() {
			return node
		}
		if itemType == "ws" {
			return root.Get("websocket")
		}
		return root.Get("http")
	default:
		return root.Get("http")
	}
}

// readFolderRequest reads the request-defaults block shared by folder.yml and
// opencollection.yml.
func readFolderRequest(folder *Folder, node *opencollection.Node) {
	folder.Headers = append(folder.Headers, keyValuesFromYAML(node.Get("headers"))...)
	folder.Auth = authFromYAML(node.Get("auth"))
	folder.VarsPre = append(folder.VarsPre, variablesFromYAML(node.Get("variables"))...)
	scripts := &Request{}
	applyYAMLScripts(scripts, node.Get("scripts"))
	folder.Scripts = scripts.Scripts
	folder.Tests = scripts.Tests
	if settings := node.Get("settings"); settings.Valid() {
		folder.Settings = settingsFromYAML(settings)
	}
}

// keyValuesFromYAML maps a list of {name, value, description, disabled} items.
func keyValuesFromYAML(node *opencollection.Node) []KeyValue {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	out := make([]KeyValue, 0, len(items))
	for _, item := range items {
		out = append(out, KeyValue{
			Name:        item.Get("name").String(),
			Value:       item.Get("value").String(),
			Enabled:     !yamlDisabled(item),
			Description: item.Get("description").String(),
		})
	}
	return out
}

// paramsFromYAML maps a params list; type defaults to query.
func paramsFromYAML(node *opencollection.Node) []Param {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	out := make([]Param, 0, len(items))
	for _, item := range items {
		kind := item.Get("type").StringOr("query")
		out = append(out, Param{
			KeyValue: KeyValue{
				Name:        item.Get("name").String(),
				Value:       item.Get("value").String(),
				Enabled:     !yamlDisabled(item),
				Description: item.Get("description").String(),
			},
			Type: kind,
		})
	}
	return out
}

// variablesFromYAML maps request or folder variables, decoding typed values.
func variablesFromYAML(node *opencollection.Node) []Variable {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	out := make([]Variable, 0, len(items))
	for _, item := range items {
		out = append(out, variableFromYAML(item))
	}
	return out
}

// variableFromYAML maps one variable, including a {type, data} value.
func variableFromYAML(node *opencollection.Node) Variable {
	v := Variable{
		Name:        node.Get("name").String(),
		Enabled:     !yamlDisabled(node),
		Kind:        VarString,
		Description: node.Get("description").String(),
	}
	value := node.Get("value")
	if value.IsMap() {
		v.Kind = VarKind(value.Get("type").StringOr(string(VarString)))
		v.Value = value.Get("data").String()
	} else {
		v.Value = value.String()
	}
	return v
}

// authFromYAML maps an auth block: `type` becomes the mode and every other scalar
// field becomes a param, in source order.
func authFromYAML(node *opencollection.Node) *Auth {
	// `auth: inherit` and `auth: none` are written as bare scalars.
	if node.IsScalar() {
		mode := node.String()
		if mode == "" {
			return nil
		}
		return &Auth{Mode: mode}
	}
	if !node.IsMap() {
		return nil
	}
	auth := &Auth{Mode: node.Get("type").String()}
	if auth.Mode == "" {
		auth.Mode = "none"
	}
	node.Each(func(key string, value *opencollection.Node) bool {
		if key == "type" {
			return true
		}
		auth.Params = append(auth.Params, KeyValue{
			Name:    key,
			Value:   value.String(),
			Enabled: true,
		})
		return true
	})
	return auth
}

// applyYAMLBody maps the http body block.
func applyYAMLBody(body *Body, node *opencollection.Node) {
	if !node.IsMap() {
		return
	}
	kind := node.Get("type").String()
	data := node.Get("data")
	if kind == "" && node.Has("query") {
		// GraphQL bodies omit `type`; the query field identifies them.
		kind = "graphql"
	}

	switch kind {
	case "form-urlencoded", "multipart-form":
		if kind == "form-urlencoded" {
			body.Mode = BodyFormURLEncoded
		} else {
			body.Mode = BodyMultipart
		}
		for _, item := range data.Items() {
			field := FormField{
				KeyValue: KeyValue{
					Name:        item.Get("name").String(),
					Value:       item.Get("value").String(),
					Enabled:     !yamlDisabled(item),
					Description: item.Get("description").String(),
				},
				Type: "text",
			}
			if files := item.Get("files").Strings(); len(files) > 0 {
				field.Type = "file"
				field.Files = files
			}
			if contentType := item.Get("contentType").String(); contentType != "" {
				field.ContentType = contentType
			}
			body.Form = append(body.Form, field)
		}
	case "file":
		body.Mode = BodyFile
		for _, item := range data.Items() {
			body.Files = append(body.Files, FileEntry{
				Path:        item.Get("path").StringOr(item.String()),
				Selected:    !yamlDisabled(item),
				ContentType: item.Get("contentType").String(),
			})
		}
	case "graphql":
		body.Mode = BodyGraphQL
		body.Text = node.Get("query").StringOr(data.String())
		body.GraphQLVars = node.Get("variables").String()
	default:
		// json, text, xml, sparql and any future raw type share one shape.
		body.Mode = BodyMode(kind)
		body.Text = data.String()
	}
}

// applyYAMLScripts maps the runtime scripts list onto the request.
func applyYAMLScripts(req *Request, node *opencollection.Node) {
	for _, item := range node.Items() {
		code := item.Get("code").String()
		switch item.Get("type").String() {
		case "before-request", "pre-request":
			req.Scripts.PreRequest = code
		case "after-response", "post-response":
			req.Scripts.PostResponse = code
		case "tests":
			req.Tests = code
		case "grpc:before-call-start", "grpc:before-message-send",
			"grpc:after-message-receive", "grpc:after-call-end":
			if req.Scripts.GRPC == nil {
				req.Scripts.GRPC = map[string]string{}
			}
			req.Scripts.GRPC[strings.TrimPrefix(item.Get("type").String(), "grpc:")] = code
		}
	}
}

// assertionsFromYAML maps the runtime assertions list.
func assertionsFromYAML(node *opencollection.Node) []Assertion {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	out := make([]Assertion, 0, len(items))
	for _, item := range items {
		out = append(out, Assertion{
			Expression:  item.Get("expression").String(),
			Operator:    item.Get("operator").StringOr("eq"),
			Operand:     item.Get("value").String(),
			Enabled:     !yamlDisabled(item),
			Description: item.Get("description").String(),
		})
	}
	return out
}

// postResponseVarsFromYAML maps set-variable actions to post-response variables.
func postResponseVarsFromYAML(node *opencollection.Node) []Variable {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	var out []Variable
	for _, item := range items {
		if item.Get("type").String() != "set-variable" {
			continue
		}
		variable := item.Get("variable")
		out = append(out, Variable{
			Name:        variable.Get("name").String(),
			Value:       item.Get("selector").Get("expression").String(),
			Enabled:     !yamlDisabled(item),
			Kind:        VarString,
			Local:       variable.Get("scope").String() == "request",
			Description: item.Get("description").String(),
		})
	}
	return out
}

// settingsFromYAML maps the settings block, keeping unset values nil.
func settingsFromYAML(node *opencollection.Node) Settings {
	var s Settings
	if v, ok := node.Get("encodeUrl").Bool(); ok {
		s.EncodeURL = v
	}
	if v, ok := node.Get("followRedirects").Bool(); ok {
		s.FollowRedirects = &v
	}
	if v, ok := node.Get("maxRedirects").Int(); ok {
		s.MaxRedirects = &v
	}
	if v, ok := node.Get("timeout").Int(); ok {
		s.TimeoutMS = v
	}
	if v, ok := node.Get("forwardAuthorizationHeader").Bool(); ok {
		s.ForwardAuthorizationHeader = &v
	}
	if v, ok := node.Get("keepAliveInterval").Int(); ok {
		s.KeepAliveInterval = &v
	}
	s.OmitHeaders = node.Get("omitHeaders").Strings()
	return s
}

// yamlDisabled reports whether an item carries `disabled: true`.
func yamlDisabled(node *opencollection.Node) bool {
	disabled, ok := node.Get("disabled").Bool()
	return ok && disabled
}

// yamlNames reads a name list that may be a single scalar or a sequence.
func yamlNames(node *opencollection.Node) []string {
	if node.IsScalar() {
		if name := node.String(); name != "" {
			return []string{name}
		}
		return nil
	}
	return node.Strings()
}

// yamlFragment renders a node back to YAML for storage in a string field.
func yamlFragment(node *opencollection.Node) string {
	doc := opencollection.NewDocument(node)
	data, err := doc.Bytes()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}
