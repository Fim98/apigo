package collection

import (
	"strconv"
	"strings"

	"apigo/internal/bru"
)

// metadataKeys are the meta fields the model understands.
var metadataKeys = map[string]bool{"name": true, "type": true, "seq": true, "tags": true}

// methodBlockNames maps a block name to the HTTP method it implies.
var methodBlockNames = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH",
	"delete": "DELETE", "head": "HEAD", "options": "OPTIONS",
	"trace": "TRACE", "connect": "CONNECT",
}

// ParseRequest parses a request file from source bytes.
func ParseRequest(path, relPath string, src []byte) (*Request, error) {
	f, err := bru.Parse(path, src)
	if err != nil {
		return nil, err
	}
	req := RequestFromFile(f)
	req.Path = path
	req.RelPath = relPath
	return req, nil
}

// RequestFromFile builds a request model from a parsed file.
func RequestFromFile(f *bru.File) *Request {
	req := &Request{}
	for _, blk := range f.Blocks {
		switch b := blk.(type) {
		case *bru.DictBlock:
			switch {
			case b.Name == "meta":
				req.Meta = metaFromBlock(b, req.Meta)
			case methodBlockNames[b.Name] != "" || b.Name == "http":
				req.applyMethodBlock(b)
			case b.Name == "params:query" || b.Name == "query":
				req.Params = append(req.Params, paramsFromPairs(b.Pairs, "query")...)
			case b.Name == "params:path":
				req.Params = append(req.Params, paramsFromPairs(b.Pairs, "path")...)
			case b.Name == "headers":
				req.Headers = append(req.Headers, keyValuesFromPairs(b.Pairs)...)
			case b.Name == "metadata":
				req.Metadata = append(req.Metadata, keyValuesFromPairs(b.Pairs)...)
			case b.Name == "auth":
				if mode, ok := b.GetString("mode"); ok {
					req.AuthMode = mode
				}
			case strings.HasPrefix(b.Name, "auth:"):
				req.Auth = authFromBlock(b, req.Auth)
			case b.Name == "vars:pre-request":
				req.VarsPre = append(req.VarsPre, variablesFromPairs(b.Pairs)...)
			case b.Name == "vars:post-response":
				req.VarsPost = append(req.VarsPost, variablesFromPairs(b.Pairs)...)
			case b.Name == "assert":
				req.Assertions = append(req.Assertions, assertionsFromPairs(b.Pairs)...)
			case strings.HasPrefix(b.Name, "body:"):
				applyBodyBlock(&req.Body, b)
			case b.Name == "settings":
				req.Settings = settingsFromBlock(b)
			default:
				req.Unknown = append(req.Unknown, blk)
			}
		case *bru.TextBlock:
			switch {
			case b.Name == "body" || b.Name == "body:json":
				req.Body.Mode = BodyJSON
				req.Body.Text = b.Text
			case b.Name == "body:text":
				req.Body.Mode = BodyText
				req.Body.Text = b.Text
			case b.Name == "body:xml":
				req.Body.Mode = BodyXML
				req.Body.Text = b.Text
			case b.Name == "body:sparql":
				req.Body.Mode = BodySPARQL
				req.Body.Text = b.Text
			case b.Name == "body:graphql":
				req.Body.Mode = BodyGraphQL
				req.Body.Text = b.Text
			case b.Name == "body:graphql:vars":
				req.Body.Mode = BodyGraphQL
				req.Body.GraphQLVars = b.Text
			case b.Name == "script:pre-request":
				req.Scripts.PreRequest = b.Text
			case b.Name == "script:post-response":
				req.Scripts.PostResponse = b.Text
			case strings.HasPrefix(b.Name, "script:grpc:"):
				if req.Scripts.GRPC == nil {
					req.Scripts.GRPC = map[string]string{}
				}
				req.Scripts.GRPC[strings.TrimPrefix(b.Name, "script:grpc:")] = b.Text
			case b.Name == "tests":
				req.Tests = b.Text
			case b.Name == "docs":
				req.Docs = b.Text
			case b.Name == "example":
				req.Examples = append(req.Examples, exampleFromText(b))
			default:
				req.Unknown = append(req.Unknown, blk)
			}
		default:
			req.Unknown = append(req.Unknown, blk)
		}
	}

	if req.Auth == nil && req.AuthMode != "" {
		req.Auth = &Auth{Mode: req.AuthMode}
	}
	if req.Body.Mode == "" {
		req.Body.Mode = BodyNone
	}
	return req
}

// applyMethodBlock reads a verb block (or the custom http block).
func (r *Request) applyMethodBlock(b *bru.DictBlock) {
	if method, ok := methodBlockNames[b.Name]; ok {
		r.Method = method
	} else if m, ok := b.GetString("method"); ok {
		r.Method = strings.ToUpper(m)
	}
	if url, ok := b.GetString("url"); ok {
		r.URL = url
	}
	if body, ok := b.GetString("body"); ok {
		r.BodyMode = body
	}
	if auth, ok := b.GetString("auth"); ok {
		r.AuthMode = auth
	}
}

// metaFromBlock reads a meta block, keeping unknown keys.
func metaFromBlock(b *bru.DictBlock, meta Meta) Meta {
	for _, p := range b.Pairs {
		switch p.Key {
		case "name":
			meta.Name = p.Value.String()
		case "type":
			meta.Type = p.Value.String()
		case "seq":
			if n, err := strconv.Atoi(strings.TrimSpace(p.Value.String())); err == nil {
				meta.Seq = n
			}
		case "tags":
			meta.Tags = append(meta.Tags, p.Value.Items...)
		default:
			meta.Extra = append(meta.Extra, p)
		}
	}
	if meta.Type == "" {
		meta.Type = string(TypeHTTP)
	}
	if meta.Seq == 0 {
		meta.Seq = 1
	}
	return meta
}

// keyValuesFromPairs converts AST pairs into model pairs, applying
// @description annotations.
func keyValuesFromPairs(pairs []bru.Pair) []KeyValue {
	out := make([]KeyValue, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, KeyValue{
			Name:        p.Key,
			Value:       p.Value.String(),
			Enabled:     !p.Disabled,
			Description: descriptionOf(p.Annotations),
			Annotations: p.Annotations,
		})
	}
	return out
}

// paramsFromPairs converts pairs into typed parameters.
func paramsFromPairs(pairs []bru.Pair, kind string) []Param {
	kvs := keyValuesFromPairs(pairs)
	out := make([]Param, 0, len(kvs))
	for _, kv := range kvs {
		out = append(out, Param{KeyValue: kv, Type: kind})
	}
	return out
}

// variablesFromPairs converts pairs into variables, reading the leading '@'
// local marker and the type annotations.
func variablesFromPairs(pairs []bru.Pair) []Variable {
	out := make([]Variable, 0, len(pairs))
	for _, p := range pairs {
		v := Variable{
			Name:        p.Key,
			Value:       p.Value.String(),
			Enabled:     !p.Disabled,
			Kind:        VarString,
			Description: descriptionOf(p.Annotations),
			Annotations: p.Annotations,
		}
		if strings.HasPrefix(v.Name, "@") {
			v.Local = true
			v.Name = strings.TrimPrefix(v.Name, "@")
		}
		for _, a := range p.Annotations {
			switch a.Name {
			case "number":
				v.Kind = VarNumber
			case "boolean":
				v.Kind = VarBoolean
			case "object":
				v.Kind = VarObject
			}
		}
		out = append(out, v)
	}
	return out
}

// assertionsFromPairs splits "op operand" into its parts.
func assertionsFromPairs(pairs []bru.Pair) []Assertion {
	out := make([]Assertion, 0, len(pairs))
	for _, p := range pairs {
		text := strings.TrimSpace(p.Value.String())
		op, operand := "eq", text
		if i := strings.IndexAny(text, " \t"); i > 0 {
			op = text[:i]
			operand = strings.TrimSpace(text[i+1:])
		} else if text != "" {
			operand = text
		}
		if text == "" {
			op = "eq"
		}
		out = append(out, Assertion{
			Expression:  p.Key,
			Operator:    op,
			Operand:     operand,
			Enabled:     !p.Disabled,
			Description: descriptionOf(p.Annotations),
		})
	}
	return out
}

// authFromBlock folds an auth:<mode> block into the auth model.
func authFromBlock(b *bru.DictBlock, existing *Auth) *Auth {
	if existing == nil {
		existing = &Auth{}
	}
	if existing.Mode == "" {
		existing.Mode = strings.TrimPrefix(b.Name, "auth:")
	}

	const additionalPrefix = "auth:oauth2:additional_params:"
	if strings.HasPrefix(b.Name, additionalPrefix) {
		parts := strings.Split(strings.TrimPrefix(b.Name, additionalPrefix), ":")
		if len(parts) == 2 {
			scope, placement := parts[0], parts[1]
			if existing.Additional == nil {
				existing.Additional = map[string]map[string][]KeyValue{}
			}
			if existing.Additional[scope] == nil {
				existing.Additional[scope] = map[string][]KeyValue{}
			}
			existing.Additional[scope][placement] = keyValuesFromPairs(b.Pairs)
		}
		return existing
	}

	for _, kv := range keyValuesFromPairs(b.Pairs) {
		existing.Params = append(existing.Params, kv)
	}
	return existing
}

// applyBodyBlock folds a body:<mode> dictionary block into the body model.
func applyBodyBlock(body *Body, b *bru.DictBlock) {
	switch b.Name {
	case "body:form-urlencoded":
		body.Mode = BodyFormURLEncoded
		for _, kv := range keyValuesFromPairs(b.Pairs) {
			body.Form = append(body.Form, FormField{KeyValue: kv, Type: "text"})
		}
	case "body:multipart-form":
		body.Mode = BodyMultipart
		for _, kv := range keyValuesFromPairs(b.Pairs) {
			field := FormField{KeyValue: kv, Type: "text"}
			field.Value, field.ContentType = splitContentType(field.Value)
			if paths, ok := parseFileReference(field.Value); ok {
				field.Type = "file"
				field.Files = paths
				field.Value = ""
			}
			body.Form = append(body.Form, field)
		}
	case "body:file":
		body.Mode = BodyFile
		for _, kv := range keyValuesFromPairs(b.Pairs) {
			value, contentType := splitContentType(kv.Value)
			entry := FileEntry{Selected: kv.Enabled, ContentType: contentType}
			if paths, ok := parseFileReference(value); ok && len(paths) > 0 {
				entry.Path = paths[0]
			} else {
				entry.Path = value
			}
			body.Files = append(body.Files, entry)
		}
	case "body:grpc":
		body.Mode = BodyGRPC
		body.Pairs = append(body.Pairs, keyValuesFromPairs(b.Pairs)...)
	case "body:ws":
		body.Mode = BodyWS
		body.Pairs = append(body.Pairs, keyValuesFromPairs(b.Pairs)...)
	default:
		body.Mode = BodyNone
	}
}

// splitContentType separates a trailing @contentType(...) annotation from a
// multipart or file value.
func splitContentType(value string) (rest, contentType string) {
	const marker = "@contentType("
	i := strings.LastIndex(value, marker)
	if i < 0 || !strings.HasSuffix(strings.TrimSpace(value), ")") {
		return strings.TrimSpace(value), ""
	}
	ct := value[i+len(marker) : strings.LastIndex(value, ")")]
	return strings.TrimSpace(value[:i]), strings.TrimSpace(ct)
}

// parseFileReference recognizes @file(a|b) and returns the paths.
func parseFileReference(value string) ([]string, bool) {
	if !strings.HasPrefix(value, "@file(") || !strings.HasSuffix(value, ")") {
		return nil, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "@file("), ")")
	var paths []string
	for _, part := range strings.Split(inner, "|") {
		if part = strings.TrimSpace(part); part != "" {
			paths = append(paths, part)
		}
	}
	return paths, true
}

// settingsFromBlock reads the settings block, keeping unset values nil.
func settingsFromBlock(b *bru.DictBlock) Settings {
	var s Settings
	for _, p := range b.Pairs {
		value := strings.TrimSpace(p.Value.String())
		switch p.Key {
		case "encodeUrl":
			s.EncodeURL = value == "true"
		case "followRedirects":
			v := value == "true"
			s.FollowRedirects = &v
		case "maxRedirects":
			if n, err := strconv.Atoi(value); err == nil {
				s.MaxRedirects = &n
			}
		case "timeout":
			if value == "inherit" {
				s.TimeoutInherit = true
			} else if n, err := strconv.Atoi(value); err == nil {
				s.TimeoutMS = n
			}
		case "forwardAuthorizationHeader":
			v := value == "true"
			s.ForwardAuthorizationHeader = &v
		case "omitHeaders":
			s.OmitHeaders = append(s.OmitHeaders, p.Value.Items...)
		case "keepAliveInterval":
			if n, err := strconv.Atoi(value); err == nil {
				s.KeepAliveInterval = &n
			}
		default:
			s.Extra = append(s.Extra, p)
		}
	}
	return s
}

// descriptionOf returns the last @description annotation's argument.
func descriptionOf(annotations []bru.Annotation) string {
	for i := len(annotations) - 1; i >= 0; i-- {
		if annotations[i].Name == "description" {
			return annotations[i].Arg
		}
	}
	return ""
}

// exampleFromText derives an example's display name from its content.
func exampleFromText(b *bru.TextBlock) Example {
	example := Example{Content: b.Text}
	for _, line := range strings.Split(b.Text, "\n") {
		trimmed := strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(trimmed, "name:"); ok {
			example.Name = strings.TrimSpace(name)
			break
		}
	}
	return example
}

// FolderFromFile builds folder defaults from a folder.bru or collection.bru.
func FolderFromFile(f *bru.File) *Folder {
	folder := &Folder{}
	for _, blk := range f.Blocks {
		switch b := blk.(type) {
		case *bru.DictBlock:
			switch {
			case b.Name == "meta":
				folder.Meta = metaFromBlock(b, folder.Meta)
			case b.Name == "headers":
				folder.Headers = append(folder.Headers, keyValuesFromPairs(b.Pairs)...)
			case b.Name == "auth":
				if mode, ok := b.GetString("mode"); ok {
					if folder.Auth == nil {
						folder.Auth = &Auth{}
					}
					folder.Auth.Mode = mode
				}
			case strings.HasPrefix(b.Name, "auth:"):
				folder.Auth = authFromBlock(b, folder.Auth)
			case b.Name == "vars:pre-request":
				folder.VarsPre = append(folder.VarsPre, variablesFromPairs(b.Pairs)...)
			case b.Name == "settings":
				folder.Settings = settingsFromBlock(b)
			default:
				folder.Settings.Extra = append(folder.Settings.Extra, b.Pairs...)
			}
		case *bru.TextBlock:
			switch b.Name {
			case "script:pre-request":
				folder.Scripts.PreRequest = b.Text
			case "script:post-response":
				folder.Scripts.PostResponse = b.Text
			case "tests":
				folder.Tests = b.Text
			case "docs":
				folder.Docs = b.Text
			}
		}
	}
	return folder
}
