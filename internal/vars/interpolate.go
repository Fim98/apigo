package vars

import (
	"strings"

	"apigo/internal/collection"
)

// defaultMaxDepth bounds nested interpolation. A chain deeper than this keeps the
// remaining tokens literal instead of recursing forever; Bruno has no such bound.
const defaultMaxDepth = 10

// maxDepthLimit caps a caller-supplied MaxDepth. A value like `a={{b}}{{b}}`
// doubles at every level, so an unbounded depth is a cheap denial of service on a
// hostile collection.
const maxDepthLimit = 100

// MaskedValue is what a secret is replaced with when MaskSecrets is set. It is
// the same marker the CLI and the UI use.
const MaskedValue = "********"

const (
	tokenOpen    = "{{"
	tokenClose   = "}}"
	promptPrefix = "?"
	dynamicMark  = "$"
)

// InterpolateOptions tunes one interpolation pass.
type InterpolateOptions struct {
	// EscapeJSON escapes each substituted value for a JSON string context:
	// backslash, quote, newline, carriage return and tab.
	EscapeJSON bool
	// Dynamic resolves `{{$name}}` dynamic variables (M2-03). Nil leaves them
	// unresolved, which is the behaviour before the faker set exists.
	Dynamic func(name string) (string, bool)
	// MaxDepth overrides the default depth bound. Values above maxDepthLimit are
	// clamped to it.
	MaxDepth int
	// MaskSecrets substitutes MaskedValue for any secret variable and sets
	// Result.Secret, so a caller can print the result without leaking a
	// credential.
	MaskSecrets bool
}

// Result is the outcome of interpolating one string.
type Result struct {
	// Text is the interpolated string. Unresolved tokens stay as written.
	Text string
	// Kind is the type of the referenced value when the whole input was a single
	// token; otherwise it is a string.
	Kind collection.VarKind
	// Unresolved lists the names that could not be resolved, in first-seen order
	// and without duplicates. A cycle or a depth cutoff also lands here.
	Unresolved []string
	// Secret reports that a secret variable took part in the result. With
	// MaskSecrets its text was replaced by MaskedValue.
	Secret bool
}

// Interpolate replaces `{{...}}` tokens in text with values from the set.
//
// Grammar and behaviour follow REQUEST_ENGINE.md §3: a token is `{{name}}` with
// dot/array paths, `{{process.env.NAME}}`, `{{$oauth2.<id>.<field>}}`,
// `{{?Prompt}}` and (once M2-03 lands) `{{$dynamic}}`. The name is looked up
// exactly as written — Bruno does not trim inside the braces, so `{{ name }}`
// looks up " name " and stays unresolved.
//
// An unresolved token is kept literally and reported. A resolved value is
// interpolated again if it contains tokens, bounded by MaxDepth and by a cycle
// check on the names in flight; a cycle keeps the token literal and reports it.
// When the whole input is one token the result keeps the referenced value's type.
func (s *Set) Interpolate(text string, opts InterpolateOptions) Result {
	it := &interpolator{set: s, opts: opts, maxDepth: opts.MaxDepth}
	if it.maxDepth <= 0 {
		it.maxDepth = defaultMaxDepth
	}
	if it.maxDepth > maxDepthLimit {
		it.maxDepth = maxDepthLimit
	}

	if name, ok := wholeToken(text); ok {
		value, found := it.resolve(name)
		if !found {
			it.note(name)
			return Result{Text: text, Kind: collection.VarString, Unresolved: it.unresolved}
		}
		if value.Secret {
			it.secret = true
			if opts.MaskSecrets {
				return Result{Text: MaskedValue, Kind: collection.VarString, Secret: true, Unresolved: it.unresolved}
			}
		}
		out := value.Text
		if strings.Contains(out, tokenOpen) {
			out = it.expand(out, []string{name}, 1, false)
		}
		kind := value.Kind
		if opts.EscapeJSON {
			// An escaped value is a string, whatever it held before.
			out = escapeJSONString(out)
			kind = collection.VarString
		}
		return Result{Text: out, Kind: kind, Secret: it.secret, Unresolved: it.unresolved}
	}

	return Result{
		Text:       it.expand(text, nil, 0, opts.EscapeJSON),
		Kind:       collection.VarString,
		Secret:     it.secret,
		Unresolved: it.unresolved,
	}
}

// interpolator carries the state of one Interpolate call.
type interpolator struct {
	set        *Set
	opts       InterpolateOptions
	maxDepth   int
	unresolved []string
	seen       map[string]bool
	// dynamic memoises {{$name}} so a token repeated in one string yields the
	// same value, which REQUEST_ENGINE §3 requires.
	dynamic map[string]dynamicResult
	secret  bool
}

// dynamicResult is one memoised dynamic lookup.
type dynamicResult struct {
	text string
	ok   bool
}

// note records an unresolved name once, keeping first-seen order.
func (i *interpolator) note(name string) {
	if i.seen == nil {
		i.seen = make(map[string]bool, 4)
	}
	if i.seen[name] {
		return
	}
	i.seen[name] = true
	i.unresolved = append(i.unresolved, name)
}

// noteAll records every token name in text without resolving it. Used when a
// nested value is left unexpanded because of the depth bound.
func (i *interpolator) noteAll(text string) {
	for _, name := range tokenNames(text) {
		i.note(name)
	}
}

// resolve looks a token name up. `?` selects the prompt layer by name and `$`
// selects a dynamic variable; everything else goes through the scope lookup,
// which already understands process.env and dot paths.
func (i *interpolator) resolve(name string) (Value, bool) {
	if name == "" {
		return Value{}, false
	}
	switch name[0] {
	case '?':
		// The `?` marks the prompt layer; answers are keyed by the plain name.
		prompt := name[len(promptPrefix):]
		if prompt == "" {
			return Value{}, false
		}
		return i.set.Lookup(prompt)
	case '$':
		// `$oauth2.<id>.<field>` is a variable name, not a dynamic function, so
		// an exact match wins before the dynamic resolver is asked.
		if value, ok := i.set.Lookup(name); ok {
			return value, true
		}
		key := name[len(dynamicMark):]
		if key == "" || i.opts.Dynamic == nil {
			return Value{}, false
		}
		if cached, ok := i.dynamic[key]; ok {
			return Value{Text: cached.text, Kind: collection.VarString}, cached.ok
		}
		text, ok := i.opts.Dynamic(key)
		if i.dynamic == nil {
			i.dynamic = make(map[string]dynamicResult, 4)
		}
		i.dynamic[key] = dynamicResult{text: text, ok: ok}
		if !ok {
			return Value{}, false
		}
		return Value{Text: text, Kind: collection.VarString}, true
	}
	return i.set.Lookup(name)
}

// expand replaces every token in text, following nested references. escape
// applies only to the outermost call: a nested value is escaped once, when it is
// inserted into the original string.
func (i *interpolator) expand(text string, stack []string, depth int, escape bool) string {
	if !strings.Contains(text, tokenOpen) {
		return text
	}

	var out strings.Builder
	out.Grow(len(text) + 16)
	pos := 0
	for pos < len(text) {
		start := strings.Index(text[pos:], tokenOpen)
		if start < 0 {
			out.WriteString(text[pos:])
			return out.String()
		}
		start += pos
		out.WriteString(text[pos:start])

		rest := text[start+len(tokenOpen):]
		end := strings.Index(rest, tokenClose)
		if end < 0 {
			// An opening brace with no close is literal text.
			out.WriteString(text[start:])
			return out.String()
		}
		name := rest[:end]
		next := start + len(tokenOpen) + end + len(tokenClose)

		if name == "" || strings.Contains(name, "}") {
			// `{{}}` and names with a closing brace are not tokens at all.
			out.WriteString(tokenOpen)
			pos = start + len(tokenOpen)
			continue
		}

		value, ok := i.resolve(name)
		if !ok || containsName(stack, name) {
			// Missing, or a cycle: keep the token as written.
			i.note(name)
			out.WriteString(text[start:next])
			pos = next
			continue
		}

		replacement := value.Text
		if value.Secret {
			i.secret = true
			if i.opts.MaskSecrets {
				replacement = MaskedValue
			}
		}
		if strings.Contains(replacement, tokenOpen) {
			if depth >= i.maxDepth {
				i.noteAll(replacement)
			} else {
				replacement = i.expand(replacement, append(stack, name), depth+1, false)
			}
		}
		if escape {
			replacement = escapeJSONString(replacement)
		}
		out.WriteString(replacement)
		pos = next
	}
	return out.String()
}

// wholeToken reports whether text is exactly one token and returns its name.
func wholeToken(text string) (string, bool) {
	if len(text) < len(tokenOpen)+len(tokenClose)+1 {
		return "", false
	}
	if !strings.HasPrefix(text, tokenOpen) || !strings.HasSuffix(text, tokenClose) {
		return "", false
	}
	inner := text[len(tokenOpen) : len(text)-len(tokenClose)]
	// The name ends at the first `}}`, so it may not itself contain one. `{` is
	// allowed, which is how `{{{a}}}` names `{a` — same as Bruno's pattern.
	if inner == "" || strings.Contains(inner, "}") {
		return "", false
	}
	return inner, true
}

// tokenNames returns the names of the well-formed tokens in text.
func tokenNames(text string) []string {
	var names []string
	pos := 0
	for pos < len(text) {
		start := strings.Index(text[pos:], tokenOpen)
		if start < 0 {
			return names
		}
		start += pos
		rest := text[start+len(tokenOpen):]
		end := strings.Index(rest, tokenClose)
		if end < 0 {
			return names
		}
		if name := rest[:end]; name != "" {
			names = append(names, name)
		}
		pos = start + len(tokenOpen) + end + len(tokenClose)
	}
	return names
}

// containsName reports whether name is already being expanded.
func containsName(stack []string, name string) bool {
	for _, seen := range stack {
		if seen == name {
			return true
		}
	}
	return false
}

// escapeJSONString escapes a value that is being substituted inside a JSON
// string. The set and order match Bruno's escapeJSONString.
//
// The loop is byte-wise on purpose: ranging over the string would replace an
// invalid UTF-8 byte with U+FFFD, silently corrupting a value that came from a
// file with the wrong encoding.
func escapeJSONString(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 8)
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteByte(text[i])
		}
	}
	return b.String()
}
