package collection

import (
	"strconv"
	"strings"

	"apigo/internal/bru"
)

// standardMethods are the HTTP methods that have their own block name.
var standardMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "TRACE": true, "CONNECT": true,
}

// RequestToFile converts a request model into an AST ready for printing.
func RequestToFile(r *Request) *bru.File {
	f := &bru.File{Path: r.Path}

	f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "meta", Pairs: metaPairs(r.Meta, nil)})

	method := strings.ToUpper(r.Method)
	blockName := strings.ToLower(method)
	pairs := []bru.Pair{}
	if !standardMethods[method] {
		blockName = "http"
		pairs = append(pairs, stringPair("method", method))
	}
	pairs = append(pairs, stringPair("url", r.URL))
	if r.BodyMode != "" && r.BodyMode != string(BodyNone) {
		pairs = append(pairs, stringPair("body", r.BodyMode))
	}
	if r.AuthMode != "" && r.AuthMode != "none" {
		pairs = append(pairs, stringPair("auth", r.AuthMode))
	}
	f.Blocks = append(f.Blocks, &bru.DictBlock{Name: blockName, Pairs: pairs})

	var query, path []bru.Pair
	for _, p := range r.Params {
		if p.Type == "path" {
			path = append(path, kvPair(p.KeyValue))
		} else {
			query = append(query, kvPair(p.KeyValue))
		}
	}
	if len(query) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "params:query", Pairs: query})
	}
	if len(path) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "params:path", Pairs: path})
	}

	if len(r.Headers) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "headers", Pairs: kvPairs(r.Headers)})
	}
	if len(r.Metadata) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "metadata", Pairs: kvPairs(r.Metadata)})
	}

	f.Blocks = append(f.Blocks, authBlocks(r.Auth)...)

	switch r.Body.Mode {
	case BodyJSON, BodyText, BodyXML, BodySPARQL, BodyGraphQL:
		name := "body:" + string(r.Body.Mode)
		if r.Body.Text != "" || r.Body.Mode == BodyJSON {
			f.Blocks = append(f.Blocks, &bru.TextBlock{Name: name, Text: r.Body.Text})
		}
		if r.Body.Mode == BodyGraphQL && r.Body.GraphQLVars != "" {
			f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "body:graphql:vars", Text: r.Body.GraphQLVars})
		}
	case BodyFormURLEncoded:
		pairs := make([]bru.Pair, 0, len(r.Body.Form))
		for _, field := range r.Body.Form {
			pairs = append(pairs, kvPair(field.KeyValue))
		}
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "body:form-urlencoded", Pairs: pairs})
	case BodyMultipart:
		pairs := make([]bru.Pair, 0, len(r.Body.Form))
		for _, field := range r.Body.Form {
			value := field.Value
			if field.Type == "file" {
				value = "@file(" + strings.Join(field.Files, "|") + ")"
			}
			if field.ContentType != "" {
				value += " @contentType(" + field.ContentType + ")"
			}
			pair := kvPair(field.KeyValue)
			pair.Value = bru.Value{Kind: bru.ValueString, Text: value}
			pairs = append(pairs, pair)
		}
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "body:multipart-form", Pairs: pairs})
	case BodyFile:
		pairs := make([]bru.Pair, 0, len(r.Body.Files))
		for i, entry := range r.Body.Files {
			value := "@file(" + entry.Path + ")"
			if entry.ContentType != "" {
				value += " @contentType(" + entry.ContentType + ")"
			}
			pairs = append(pairs, bru.Pair{
				Key:      "file" + strconv.Itoa(i+1),
				Disabled: !entry.Selected,
				Value:    bru.Value{Kind: bru.ValueString, Text: value},
			})
		}
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "body:file", Pairs: pairs})
	case BodyGRPC, BodyWS:
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "body:" + string(r.Body.Mode), Pairs: kvPairs(r.Body.Pairs)})
	}

	if len(r.VarsPre) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "vars:pre-request", Pairs: variablePairs(r.VarsPre)})
	}
	if len(r.VarsPost) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "vars:post-response", Pairs: variablePairs(r.VarsPost)})
	}
	if len(r.Assertions) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "assert", Pairs: assertionPairs(r.Assertions), AssertSyntax: true})
	}

	if r.Scripts.PreRequest != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "script:pre-request", Text: r.Scripts.PreRequest})
	}
	if r.Scripts.PostResponse != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "script:post-response", Text: r.Scripts.PostResponse})
	}
	for _, hook := range []string{"before-call-start", "before-message-send", "after-message-receive", "after-call-end"} {
		if src := r.Scripts.GRPC[hook]; src != "" {
			f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "script:grpc:" + hook, Text: src})
		}
	}
	if r.Tests != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "tests", Text: r.Tests})
	}
	if pairs := settingsPairs(r.Settings); len(pairs) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "settings", Pairs: pairs})
	}
	if r.Docs != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "docs", Text: r.Docs})
	}

	f.Blocks = append(f.Blocks, r.Unknown...)

	for _, example := range r.Examples {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "example", Text: example.Content, Example: true})
	}
	return f
}

// FolderToFile converts folder or collection defaults into an AST.
func FolderToFile(folder *Folder) *bru.File {
	f := &bru.File{Path: folder.Path}
	if folder.Meta.Name != "" || len(folder.Meta.Extra) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "meta", Pairs: metaPairs(folder.Meta, nil)})
	}
	if len(folder.Headers) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "headers", Pairs: kvPairs(folder.Headers)})
	}
	f.Blocks = append(f.Blocks, authBlocks(folder.Auth)...)
	if len(folder.VarsPre) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "vars:pre-request", Pairs: variablePairs(folder.VarsPre)})
	}
	if folder.Scripts.PreRequest != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "script:pre-request", Text: folder.Scripts.PreRequest})
	}
	if folder.Scripts.PostResponse != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "script:post-response", Text: folder.Scripts.PostResponse})
	}
	if pairs := settingsPairs(folder.Settings); len(pairs) > 0 {
		f.Blocks = append(f.Blocks, &bru.DictBlock{Name: "settings", Pairs: pairs})
	}
	if folder.Tests != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "tests", Text: folder.Tests})
	}
	if folder.Docs != "" {
		f.Blocks = append(f.Blocks, &bru.TextBlock{Name: "docs", Text: folder.Docs})
	}
	return f
}

// MarshalRequest renders a request in canonical .bru form.
func MarshalRequest(r *Request) []byte { return bru.Format(RequestToFile(r)) }

// MarshalFolder renders folder or collection defaults in canonical .bru form.
func MarshalFolder(f *Folder) []byte { return bru.Format(FolderToFile(f)) }

func metaPairs(meta Meta, extra []bru.Pair) []bru.Pair {
	var pairs []bru.Pair
	if meta.Name != "" {
		pairs = append(pairs, stringPair("name", meta.Name))
	}
	if meta.Type != "" {
		pairs = append(pairs, stringPair("type", meta.Type))
	}
	if meta.Seq != 0 {
		pairs = append(pairs, stringPair("seq", strconv.Itoa(meta.Seq)))
	}
	if len(meta.Tags) > 0 {
		pairs = append(pairs, bru.Pair{Key: "tags", Value: bru.Value{Kind: bru.ValueList, Items: meta.Tags}})
	}
	return append(pairs, append(meta.Extra, extra...)...)
}

func stringPair(key, value string) bru.Pair {
	return bru.Pair{Key: key, Value: bru.Value{Kind: bru.ValueString, Text: value}}
}

func kvPair(kv KeyValue) bru.Pair {
	return bru.Pair{
		Key:         kv.Name,
		Disabled:    !kv.Enabled,
		Annotations: kv.Annotations,
		Value:       bru.Value{Kind: bru.ValueString, Text: kv.Value},
	}
}

func kvPairs(kvs []KeyValue) []bru.Pair {
	out := make([]bru.Pair, 0, len(kvs))
	for _, kv := range kvs {
		out = append(out, kvPair(kv))
	}
	return out
}

func variablePairs(vars []Variable) []bru.Pair {
	out := make([]bru.Pair, 0, len(vars))
	for _, v := range vars {
		name := v.Name
		if v.Local {
			name = "@" + name
		}
		annotations := v.Annotations
		if len(annotations) == 0 && v.Kind != "" && v.Kind != VarString {
			annotations = []bru.Annotation{{Name: string(v.Kind)}}
		}
		out = append(out, bru.Pair{
			Key:         name,
			Disabled:    !v.Enabled,
			Annotations: annotations,
			Value:       bru.Value{Kind: bru.ValueString, Text: v.Value},
		})
	}
	return out
}

func assertionPairs(assertions []Assertion) []bru.Pair {
	out := make([]bru.Pair, 0, len(assertions))
	for _, a := range assertions {
		value := strings.TrimSpace(a.Operator + " " + a.Operand)
		out = append(out, bru.Pair{
			Key:      a.Expression,
			Disabled: !a.Enabled,
			Value:    bru.Value{Kind: bru.ValueString, Text: value},
		})
	}
	return out
}

func settingsPairs(s Settings) []bru.Pair {
	var pairs []bru.Pair
	pairs = append(pairs, stringPair("encodeUrl", strconv.FormatBool(s.EncodeURL)))
	if s.FollowRedirects != nil {
		pairs = append(pairs, stringPair("followRedirects", strconv.FormatBool(*s.FollowRedirects)))
	}
	if s.MaxRedirects != nil {
		pairs = append(pairs, stringPair("maxRedirects", strconv.Itoa(*s.MaxRedirects)))
	}
	switch {
	case s.TimeoutInherit:
		pairs = append(pairs, stringPair("timeout", "inherit"))
	case s.TimeoutMS != 0:
		pairs = append(pairs, stringPair("timeout", strconv.Itoa(s.TimeoutMS)))
	}
	if s.ForwardAuthorizationHeader != nil {
		pairs = append(pairs, stringPair("forwardAuthorizationHeader", strconv.FormatBool(*s.ForwardAuthorizationHeader)))
	}
	if len(s.OmitHeaders) > 0 {
		pairs = append(pairs, bru.Pair{Key: "omitHeaders", Value: bru.Value{Kind: bru.ValueList, Items: s.OmitHeaders}})
	}
	if s.KeepAliveInterval != nil {
		pairs = append(pairs, stringPair("keepAliveInterval", strconv.Itoa(*s.KeepAliveInterval)))
	}
	return append(pairs, s.Extra...)
}

// authBlocks converts an auth model into the auth:<mode> blocks, including the
// OAuth2 additional-parameter blocks.
func authBlocks(auth *Auth) []bru.Block {
	if auth == nil || auth.Mode == "" || auth.Mode == "none" {
		return nil
	}
	var blocks []bru.Block
	blocks = append(blocks, &bru.DictBlock{Name: "auth:" + auth.Mode, Pairs: kvPairs(auth.Params)})
	if len(auth.Additional) > 0 {
		for _, scope := range []string{"auth_req", "access_token_req", "refresh_token_req"} {
			placements, ok := auth.Additional[scope]
			if !ok {
				continue
			}
			for _, placement := range []string{"headers", "queryparams", "body"} {
				pairs, ok := placements[placement]
				if !ok {
					continue
				}
				blocks = append(blocks, &bru.DictBlock{
					Name:  "auth:oauth2:additional_params:" + scope + ":" + placement,
					Pairs: kvPairs(pairs),
				})
			}
		}
	}
	return blocks
}
