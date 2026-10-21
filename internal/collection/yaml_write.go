package collection

import (
	"strings"

	"apigo/internal/opencollection"
)

// Key sets the writer owns inside list items. Merging keeps everything else, so
// a vendor field inside a header or a variable survives a save.
var (
	ownedKeyValue  = []string{"name", "value", "disabled", "description"}
	ownedParam     = []string{"name", "value", "disabled", "description", "type"}
	ownedVariable  = []string{"name", "value", "disabled", "description"}
	ownedAction    = []string{"type", "phase", "selector", "variable", "disabled", "description"}
	ownedAssertion = []string{"expression", "operator", "value", "disabled", "description"}
	ownedScript    = []string{"type", "code"}
	ownedFormEntry = []string{"name", "value", "disabled", "description", "files", "contentType"}
	ownedFileEntry = []string{"path", "contentType", "disabled"}
)

// MarshalYAMLRequest renders a request as OpenCollection YAML.
//
// When the request was read from a YAML file, the writer patches a clone of the
// original document: keys the model does not cover stay where they were, at the
// block level and inside list items. A request built in memory gets a fresh
// document.
func MarshalYAMLRequest(req *Request) ([]byte, error) {
	doc := baseDocument(req.Source)
	root := doc.Root()
	writeYAMLInfo(root, req.Meta, "")

	proto := ensureMap(root, yamlProtocolKey(req.Meta.Type))
	setIf(proto, "method", strings.ToUpper(req.Method))
	setIf(proto, "url", req.URL)
	mergeList(proto, "headers", keyValueNodes(req.Headers), ownedKeyValue)
	mergeList(proto, "params", paramNodes(req.Params), ownedParam)
	mergeList(proto, "metadata", keyValueNodes(req.Metadata), ownedKeyValue)
	writeYAMLAuth(proto, req.Auth)
	writeYAMLBody(proto, req.Body)

	writeYAMLRuntime(root, req.VarsPre, req.VarsPost, req.Assertions, req.Scripts, req.Tests)
	writeYAMLSettings(root, req.Settings)
	writeYAMLDocs(root, req.Docs)
	writeYAMLExamples(root, req)
	return doc.Bytes()
}

// MarshalYAMLFolder renders folder defaults as OpenCollection YAML.
func MarshalYAMLFolder(folder *Folder) ([]byte, error) {
	doc := baseDocument(folder.Source)
	root := doc.Root()
	writeYAMLInfo(root, folder.Meta, "folder")

	if hasFolderDefaults(folder) || root.Has("request") {
		writeYAMLDefaults(ensureMap(root, "request"), folder)
	}
	writeYAMLDocs(root, folder.Docs)
	return doc.Bytes()
}

// MarshalYAMLCollection renders the collection root file.
func MarshalYAMLCollection(c *Collection) ([]byte, error) {
	doc := baseDocument(c.Source)
	root := doc.Root()
	if !root.Has("opencollection") {
		root.Prepend("opencollection", "1.0.0")
	}

	info := ensureMap(root, "info")
	setIf(info, "name", c.Config.Name)
	setIf(info, "version", c.Config.Version)

	if c.Defaults != nil && (hasFolderDefaults(c.Defaults) || root.Has("request")) {
		writeYAMLDefaults(ensureMap(root, "request"), c.Defaults)
	}

	if len(c.Config.Ignore) > 0 {
		bruno := ensureMap(ensureMap(root, "extensions"), "bruno")
		mergeList(bruno, "ignore", stringNodes(c.Config.Ignore), nil)
	} else if root.Get("extensions").IsMap() {
		root.Get("extensions").Get("bruno").Delete("ignore")
	}
	return doc.Bytes()
}

// MarshalYAMLEnvironment renders an environment file.
func MarshalYAMLEnvironment(env *Environment) ([]byte, error) {
	doc := baseDocument(env.Source)
	root := doc.Root()
	setIf(root, "name", env.Name)
	switch len(env.Extends) {
	case 0:
		root.Delete("extends")
	case 1:
		_ = root.Set("extends", env.Extends[0])
	default:
		mergeList(root, "extends", stringNodes(env.Extends), nil)
	}
	setIf(root, "color", env.Color)

	secrets := make(map[string]bool, len(env.SecretNames))
	for _, name := range env.SecretNames {
		secrets[name] = true
	}
	var variables []*opencollection.Node
	for _, v := range env.Vars {
		if secrets[v.Name] {
			node := opencollection.NewMap()
			_ = node.Set("name", v.Name)
			_ = node.Set("secret", true)
			if !v.Enabled {
				_ = node.Set("disabled", true)
			}
			if v.Description != "" {
				_ = node.Set("description", v.Description)
			}
			variables = append(variables, node)
			continue
		}
		variables = append(variables, variableNode(v))
	}
	mergeList(root, "variables", variables, ownedVariable)
	return doc.Bytes()
}

// writeYAMLDefaults patches the `request:` block shared by folder.yml and the
// collection root.
func writeYAMLDefaults(node *opencollection.Node, folder *Folder) {
	mergeList(node, "headers", keyValueNodes(folder.Headers), ownedKeyValue)
	writeYAMLAuth(node, folder.Auth)
	mergeList(node, "variables", variableNodes(folder.VarsPre), ownedVariable)
	mergeList(node, "actions", actionNodes(folder.VarsPost), ownedAction)
	mergeList(node, "scripts", scriptNodes(folder.Scripts, folder.Tests), ownedScript)
	writeYAMLSettings(node, folder.Settings)
}

// hasFolderDefaults reports whether a folder carries anything to write.
func hasFolderDefaults(folder *Folder) bool {
	return len(folder.Headers) > 0 || folder.Auth != nil || len(folder.VarsPre) > 0 ||
		len(folder.VarsPost) > 0 || !folder.Scripts.IsZero() || folder.Tests != "" ||
		folder.Settings.EncodeURL || folder.Settings.TimeoutMS != 0 || !folder.Settings.IsZero()
}

// baseDocument clones a source document, or starts an empty one.
func baseDocument(src *opencollection.Document) *opencollection.Document {
	if src != nil && src.Root().Valid() {
		return src.Clone()
	}
	return opencollection.NewDocument(opencollection.NewMap())
}

// ensureMap returns key as a mapping, creating or upgrading it when needed.
func ensureMap(parent *opencollection.Node, key string) *opencollection.Node {
	if !parent.Valid() {
		return nil
	}
	child := parent.Get(key)
	if child.IsMap() {
		return child
	}
	child = opencollection.NewMap()
	_ = parent.Set(key, child)
	return child
}

// setIf sets a scalar when value is non-empty and removes the key otherwise.
func setIf(parent *opencollection.Node, key, value string) {
	if !parent.Valid() {
		return
	}
	if value == "" {
		parent.Delete(key)
		return
	}
	_ = parent.Set(key, value)
}

// mergeList writes a list, keeping unknown keys of items that already exist at
// the same index. A different length replaces the list wholesale, because there
// is no stable identity to merge on.
func mergeList(parent *opencollection.Node, key string, items []*opencollection.Node, owned []string) {
	if !parent.Valid() {
		return
	}
	if len(items) == 0 {
		parent.Delete(key)
		return
	}
	existing := parent.Get(key)
	if !existing.IsSequence() || existing.Len() != len(items) {
		seq := opencollection.NewSequence()
		seq.Append(items...)
		_ = parent.Set(key, seq)
		return
	}
	current := existing.Items()
	for i, next := range items {
		target := current[i]
		if !target.IsMap() || owned == nil {
			existing.SetAt(i, next)
			continue
		}
		mergeMap(target, next, owned)
	}
}

// mergeMap copies next's keys onto target and drops the owned keys the model no
// longer has, leaving every other key untouched.
func mergeMap(target, next *opencollection.Node, owned []string) {
	next.Each(func(key string, value *opencollection.Node) bool {
		_ = target.Set(key, value)
		return true
	})
	for _, key := range owned {
		if !next.Has(key) {
			target.Delete(key)
		}
	}
}

// yamlProtocolKey is the block name for a request type.
func yamlProtocolKey(requestType string) string {
	switch RequestType(requestType) {
	case TypeGraphQL:
		return "graphql"
	case TypeGRPC:
		return "grpc"
	case TypeWS:
		return "websocket"
	default:
		return "http"
	}
}

// writeYAMLInfo patches the info block. typeOverride wins over the model's type,
// which is how folders keep `type: folder`.
func writeYAMLInfo(root *opencollection.Node, meta Meta, typeOverride string) {
	info := ensureMap(root, "info")
	setIf(info, "name", meta.Name)
	itemType := meta.Type
	if typeOverride != "" {
		itemType = typeOverride
	}
	setIf(info, "type", yamlItemType(itemType))
	if meta.Seq > 0 {
		_ = info.Set("seq", meta.Seq)
	}
	mergeList(info, "tags", stringNodes(meta.Tags), nil)
	setIf(info, "description", meta.Description)
}

// yamlItemType is the type string the format uses for a model request type.
func yamlItemType(requestType string) string {
	switch requestType {
	case string(TypeWS):
		return "websocket"
	case "":
		return "http"
	default:
		return requestType
	}
}

// keyValueNodes builds {name, value, disabled, description} mappings.
func keyValueNodes(pairs []KeyValue) []*opencollection.Node {
	nodes := make([]*opencollection.Node, 0, len(pairs))
	for _, kv := range pairs {
		nodes = append(nodes, keyValueNode(kv))
	}
	return nodes
}

// keyValueNode builds one {name, value, ...} mapping.
func keyValueNode(kv KeyValue) *opencollection.Node {
	node := opencollection.NewMap()
	_ = node.Set("name", kv.Name)
	_ = node.Set("value", kv.Value)
	if !kv.Enabled {
		_ = node.Set("disabled", true)
	}
	if kv.Description != "" {
		_ = node.Set("description", kv.Description)
	}
	return node
}

// paramNodes builds params, which carry a type.
func paramNodes(params []Param) []*opencollection.Node {
	nodes := make([]*opencollection.Node, 0, len(params))
	for _, p := range params {
		node := keyValueNode(p.KeyValue)
		kind := p.Type
		if kind == "" {
			kind = "query"
		}
		_ = node.Set("type", kind)
		nodes = append(nodes, node)
	}
	return nodes
}

// variableNodes builds variables, typing values that are not strings.
func variableNodes(vars []Variable) []*opencollection.Node {
	nodes := make([]*opencollection.Node, 0, len(vars))
	for _, v := range vars {
		nodes = append(nodes, variableNode(v))
	}
	return nodes
}

// variableNode builds one variable; a non-string type becomes {type, data}.
func variableNode(v Variable) *opencollection.Node {
	node := opencollection.NewMap()
	_ = node.Set("name", v.Name)
	if v.Kind != "" && v.Kind != VarString {
		typed := opencollection.NewMap()
		_ = typed.Set("type", string(v.Kind))
		_ = typed.Set("data", v.Value)
		_ = node.Set("value", typed)
	} else {
		_ = node.Set("value", v.Value)
	}
	if !v.Enabled {
		_ = node.Set("disabled", true)
	}
	if v.Description != "" {
		_ = node.Set("description", v.Description)
	}
	return node
}

// actionNodes builds set-variable actions for post-response variables.
func actionNodes(vars []Variable) []*opencollection.Node {
	nodes := make([]*opencollection.Node, 0, len(vars))
	for _, v := range vars {
		selector := opencollection.NewMap()
		_ = selector.Set("expression", v.Value)
		_ = selector.Set("method", "jsonq")
		variable := opencollection.NewMap()
		_ = variable.Set("name", v.Name)
		scope := "runtime"
		if v.Local {
			scope = "request"
		}
		_ = variable.Set("scope", scope)

		node := opencollection.NewMap()
		_ = node.Set("type", "set-variable")
		_ = node.Set("phase", "after-response")
		_ = node.Set("selector", selector)
		_ = node.Set("variable", variable)
		if !v.Enabled {
			_ = node.Set("disabled", true)
		}
		if v.Description != "" {
			_ = node.Set("description", v.Description)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// actionNodesWithExtras keeps actions the model does not own, such as a future
// action type, instead of replacing the whole list.
func actionNodesWithExtras(existing *opencollection.Node, vars []Variable) []*opencollection.Node {
	nodes := actionNodes(vars)
	for _, item := range existing.Items() {
		if item.Get("type").String() != "set-variable" {
			nodes = append(nodes, item)
		}
	}
	return nodes
}

// scriptNodes builds the runtime.scripts list in execution order.
func scriptNodes(scripts Scripts, tests string) []*opencollection.Node {
	var nodes []*opencollection.Node
	add := func(kind, code string) {
		if code == "" {
			return
		}
		node := opencollection.NewMap()
		_ = node.Set("type", kind)
		_ = node.Set("code", code)
		nodes = append(nodes, node)
	}
	add("before-request", scripts.PreRequest)
	add("after-response", scripts.PostResponse)
	add("tests", tests)
	for _, hook := range []string{"before-call-start", "before-message-send", "after-message-receive", "after-call-end"} {
		add("grpc:"+hook, scripts.GRPC[hook])
	}
	return nodes
}

// writeYAMLAuth patches the auth block. `none` and `inherit` stay bare scalars,
// the way Bruno writes them. Nested fields the model does not read, such as OAuth2
// additional parameters, are left in place.
func writeYAMLAuth(parent *opencollection.Node, auth *Auth) {
	if !parent.Valid() {
		return
	}
	if auth == nil || auth.Mode == "" {
		parent.Delete("auth")
		return
	}
	switch auth.Mode {
	case "none", "inherit":
		_ = parent.Set("auth", auth.Mode)
		return
	}

	node := opencollection.NewMap()
	_ = node.Set("type", auth.Mode)
	for _, kv := range auth.Params {
		_ = node.Set(kv.Name, kv.Value)
	}
	for _, scope := range []string{"auth_req", "access_token_req", "refresh_token_req"} {
		placements, ok := auth.Additional[scope]
		if !ok {
			continue
		}
		additional := ensureMap(node, "additional_params")
		scopeNode := ensureMap(additional, scope)
		for _, placement := range []string{"headers", "queryparams", "body"} {
			mergeList(scopeNode, placement, keyValueNodes(placements[placement]), ownedKeyValue)
		}
	}

	existing := parent.Get("auth")
	if existing.IsMap() {
		owned := []string{"type"}
		existing.Each(func(key string, value *opencollection.Node) bool {
			if key != "type" && value.IsScalar() {
				owned = append(owned, key)
			}
			return true
		})
		mergeMap(existing, node, owned)
		return
	}
	_ = parent.Set("auth", node)
}

// writeYAMLBody patches the body block.
func writeYAMLBody(parent *opencollection.Node, body Body) {
	if !parent.Valid() {
		return
	}
	switch body.Mode {
	case "", BodyNone:
		parent.Delete("body")
		return
	case BodyGraphQL:
		if body.Text == "" && body.GraphQLVars == "" {
			parent.Delete("body")
			return
		}
		node := ensureMap(parent, "body")
		node.Delete("type")
		setIf(node, "query", body.Text)
		setIf(node, "variables", body.GraphQLVars)
		return
	}

	node := ensureMap(parent, "body")
	_ = node.Set("type", string(body.Mode))
	switch body.Mode {
	case BodyFormURLEncoded, BodyMultipart:
		kind := "form-urlencoded"
		if body.Mode == BodyMultipart {
			kind = "multipart-form"
		}
		_ = node.Set("type", kind)
		nodes := make([]*opencollection.Node, 0, len(body.Form))
		for _, field := range body.Form {
			entry := opencollection.NewMap()
			_ = entry.Set("name", field.Name)
			if field.Type == "file" {
				mergeList(entry, "files", stringNodes(field.Files), nil)
			} else {
				_ = entry.Set("value", field.Value)
			}
			if field.ContentType != "" {
				_ = entry.Set("contentType", field.ContentType)
			}
			if !field.Enabled {
				_ = entry.Set("disabled", true)
			}
			if field.Description != "" {
				_ = entry.Set("description", field.Description)
			}
			nodes = append(nodes, entry)
		}
		mergeList(node, "data", nodes, ownedFormEntry)
	case BodyFile:
		nodes := make([]*opencollection.Node, 0, len(body.Files))
		for _, file := range body.Files {
			entry := opencollection.NewMap()
			_ = entry.Set("path", file.Path)
			if file.ContentType != "" {
				_ = entry.Set("contentType", file.ContentType)
			}
			if !file.Selected {
				_ = entry.Set("disabled", true)
			}
			nodes = append(nodes, entry)
		}
		mergeList(node, "data", nodes, ownedFileEntry)
	default:
		_ = node.Set("data", body.Text)
	}
}

// writeYAMLRuntime patches the runtime block: variables, scripts, assertions and
// post-response actions. Actions the model does not own, such as a script action
// added by a newer Bruno, are kept.
func writeYAMLRuntime(root *opencollection.Node, varsPre, varsPost []Variable, assertions []Assertion, scripts Scripts, tests string) {
	hasContent := len(varsPre) > 0 || len(varsPost) > 0 || len(assertions) > 0 || !scripts.IsZero() || tests != ""
	runtime := root.Get("runtime")
	if !hasContent {
		if runtime.IsMap() {
			runtime.Delete("variables")
			runtime.Delete("scripts")
			runtime.Delete("assertions")
			runtime.Delete("actions")
		}
		return
	}

	runtime = ensureMap(root, "runtime")
	mergeList(runtime, "variables", variableNodes(varsPre), ownedVariable)
	mergeList(runtime, "scripts", scriptNodes(scripts, tests), ownedScript)
	mergeList(runtime, "actions", actionNodesWithExtras(runtime.Get("actions"), varsPost), ownedAction)

	nodes := make([]*opencollection.Node, 0, len(assertions))
	for _, a := range assertions {
		node := opencollection.NewMap()
		_ = node.Set("expression", a.Expression)
		_ = node.Set("operator", a.Operator)
		if a.Operand != "" {
			_ = node.Set("value", a.Operand)
		}
		if !a.Enabled {
			_ = node.Set("disabled", true)
		}
		if a.Description != "" {
			_ = node.Set("description", a.Description)
		}
		nodes = append(nodes, node)
	}
	mergeList(runtime, "assertions", nodes, ownedAssertion)
}

// writeYAMLSettings patches a settings block.
func writeYAMLSettings(root *opencollection.Node, s Settings) {
	node := ensureMap(root, "settings")
	_ = node.Set("encodeUrl", s.EncodeURL)
	if s.FollowRedirects != nil {
		_ = node.Set("followRedirects", *s.FollowRedirects)
	}
	if s.MaxRedirects != nil {
		_ = node.Set("maxRedirects", *s.MaxRedirects)
	}
	_ = node.Set("timeout", s.TimeoutMS)
	if s.ForwardAuthorizationHeader != nil {
		_ = node.Set("forwardAuthorizationHeader", *s.ForwardAuthorizationHeader)
	}
	if s.KeepAliveInterval != nil {
		_ = node.Set("keepAliveInterval", *s.KeepAliveInterval)
	}
	mergeList(node, "omitHeaders", stringNodes(s.OmitHeaders), nil)
}

// writeYAMLDocs patches the docs block. Unknown keys in it survive.
func writeYAMLDocs(root *opencollection.Node, content string) {
	docs := root.Get("docs")
	if content == "" {
		if !docs.IsMap() {
			return
		}
		docs.Delete("content")
		if docs.Len() == 0 {
			root.Delete("docs")
		}
		return
	}
	node := ensureMap(root, "docs")
	_ = node.Set("content", content)
	if !node.Has("type") {
		_ = node.Set("type", "text/markdown")
	}
}

// writeYAMLExamples rebuilds the examples list only when the model's count
// changed, using the stored YAML fragments. Examples read from `.bru` are not
// YAML, so a conversion leaves the source list alone.
func writeYAMLExamples(root *opencollection.Node, req *Request) {
	if req.Source == nil {
		return
	}
	if root.Get("examples").Len() == len(req.Examples) {
		return
	}
	nodes := make([]*opencollection.Node, 0, len(req.Examples))
	for _, example := range req.Examples {
		doc, err := opencollection.Parse([]byte(example.Content))
		if err != nil || !doc.Root().IsMap() {
			return
		}
		nodes = append(nodes, doc.Root())
	}
	mergeList(root, "examples", nodes, nil)
}

// stringNodes wraps strings as scalar nodes.
func stringNodes(values []string) []*opencollection.Node {
	nodes := make([]*opencollection.Node, 0, len(values))
	for _, value := range values {
		node, err := opencollection.Value(value)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}
