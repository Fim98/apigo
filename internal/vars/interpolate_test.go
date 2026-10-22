package vars

import (
	"strconv"
	"strings"
	"testing"

	"apigo/internal/collection"
)

// newSet builds a set with string variables in the request layer.
func newSet(pairs ...string) *Set {
	set := NewSet()
	variables := make([]collection.Variable, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		variables = append(variables, collection.Variable{
			Name: pairs[i], Value: pairs[i+1], Enabled: true, Kind: collection.VarString,
		})
	}
	set.Load(ScopeRequest, variables)
	return set
}

func TestInterpolatePlainText(t *testing.T) {
	set := newSet()
	tests := []struct {
		name       string
		text       string
		want       string
		unresolved []string
	}{
		{name: "empty", text: "", want: ""},
		{name: "no tokens", text: "hello", want: "hello"},
		{name: "braces without a token", text: "a { b } c", want: "a { b } c"},
		{name: "unclosed token", text: "a {{name", want: "a {{name"},
		{name: "closing braces only", text: "a }} b", want: "a }} b"},
		{name: "empty token", text: "a {{}} b", want: "a {{}} b"},
		{name: "token with an inner brace", text: "{{a}b}}", want: "{{a}b}}"},
		{
			name: "adjacent tokens", text: "{{a}}{{b}}", want: "{{a}}{{b}}",
			unresolved: []string{"a", "b"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want {
				t.Fatalf("= %q, want %q", got.Text, tc.want)
			}
			if strings.Join(got.Unresolved, ",") != strings.Join(tc.unresolved, ",") {
				t.Fatalf("unresolved = %v, want %v", got.Unresolved, tc.unresolved)
			}
		})
	}
}

func TestInterpolateBasic(t *testing.T) {
	set := newSet("host", "example.com", "scheme", "https", "path", "/v1/users")
	set.PutText(ScopeEnvironment, "region", "eu")

	tests := []struct {
		name string
		text string
		want string
	}{
		{"one token", "{{host}}", "example.com"},
		{"token in text", "https://{{host}}/x", "https://example.com/x"},
		{"adjacent tokens", "{{scheme}}://{{host}}{{path}}", "https://example.com/v1/users"},
		{"environment layer", "{{region}}", "eu"},
		{"repeated token", "{{host}} and {{host}}", "example.com and example.com"},
		{"token next to braces", "{ {{host}} }", "{ example.com }"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want {
				t.Fatalf("= %q, want %q", got.Text, tc.want)
			}
			if len(got.Unresolved) != 0 {
				t.Fatalf("unresolved = %v", got.Unresolved)
			}
		})
	}
}

func TestInterpolateUnresolved(t *testing.T) {
	set := newSet("known", "yes")

	got := set.Interpolate("{{known}} {{missing}} {{known}} {{missing}}", InterpolateOptions{})
	if got.Text != "yes {{missing}} yes {{missing}}" {
		t.Fatalf("text = %q", got.Text)
	}
	if len(got.Unresolved) != 1 || got.Unresolved[0] != "missing" {
		t.Fatalf("unresolved = %v, want one deduplicated name", got.Unresolved)
	}
	if got.Kind != collection.VarString {
		t.Fatalf("kind = %v", got.Kind)
	}
}

func TestInterpolateWholeTokenKeepsType(t *testing.T) {
	set := newSet()
	set.Load(ScopeRequest, []collection.Variable{
		{Name: "count", Value: "3", Kind: collection.VarNumber, Enabled: true},
		{Name: "enabled", Value: "true", Kind: collection.VarBoolean, Enabled: true},
		{Name: "user", Value: `{"name":"Ada"}`, Kind: collection.VarObject, Enabled: true},
		{Name: "rows", Value: `[1,2]`, Kind: collection.VarObject, Enabled: true},
	})

	tests := []struct {
		text string
		want string
		kind collection.VarKind
	}{
		{"{{count}}", "3", collection.VarNumber},
		{"{{enabled}}", "true", collection.VarBoolean},
		{"{{user}}", `{"name":"Ada"}`, collection.VarObject},
		{"{{rows}}", `[1,2]`, collection.VarObject},
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want || got.Kind != tc.kind {
				t.Fatalf("= %q (%v), want %q (%v)", got.Text, got.Kind, tc.want, tc.kind)
			}
		})
	}

	// A larger string is always a string, even when the token is typed.
	got := set.Interpolate("count={{count}}", InterpolateOptions{})
	if got.Text != "count=3" || got.Kind != collection.VarString {
		t.Fatalf("= %q (%v)", got.Text, got.Kind)
	}
}

func TestInterpolateNested(t *testing.T) {
	set := newSet(
		"host", "{{scheme}}://{{domain}}:{{port}}",
		"scheme", "https",
		"domain", "example.com",
		"port", "8443",
	)

	got := set.Interpolate("{{host}}/v1", InterpolateOptions{})
	if got.Text != "https://example.com:8443/v1" {
		t.Fatalf("text = %q", got.Text)
	}
	if len(got.Unresolved) != 0 {
		t.Fatalf("unresolved = %v", got.Unresolved)
	}
}

func TestInterpolateUnresolvedInsideNestedValue(t *testing.T) {
	set := newSet("url", "{{scheme}}://{{missing}}", "scheme", "https")

	got := set.Interpolate("{{url}}", InterpolateOptions{})
	if got.Text != "https://{{missing}}" {
		t.Fatalf("text = %q", got.Text)
	}
	// scheme resolves, missing does not: only the missing name is reported.
	if len(got.Unresolved) != 1 || got.Unresolved[0] != "missing" {
		t.Fatalf("unresolved = %v", got.Unresolved)
	}
}

func TestInterpolateCycleTerminates(t *testing.T) {
	set := newSet("a", "{{b}}", "b", "{{a}}", "self", "x{{self}}")

	tests := []struct {
		name       string
		text       string
		want       string
		unresolved []string
	}{
		{"two names", "{{a}}", "{{a}}", []string{"a"}},
		{"self reference", "{{self}}", "x{{self}}", []string{"self"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want {
				t.Fatalf("text = %q, want %q", got.Text, tc.want)
			}
			if strings.Join(got.Unresolved, ",") != strings.Join(tc.unresolved, ",") {
				t.Fatalf("unresolved = %v, want %v", got.Unresolved, tc.unresolved)
			}
		})
	}
}

func TestInterpolateDepthBound(t *testing.T) {
	// A chain deeper than the bound must stop, keep the token and report it.
	set := NewSet()
	variables := make([]collection.Variable, 0, 20)
	for i := 0; i < 20; i++ {
		variables = append(variables, collection.Variable{
			Name: "v" + strconv.Itoa(i), Value: "{{v" + strconv.Itoa(i+1) + "}}",
			Enabled: true, Kind: collection.VarString,
		})
	}
	variables = append(variables, collection.Variable{
		Name: "v20", Value: "end", Enabled: true, Kind: collection.VarString,
	})
	set.Load(ScopeRequest, variables)

	// MaxDepth counts expansions: three levels resolve v1, v2 and v3, so the
	// token left over is v4.
	got := set.Interpolate("{{v0}}", InterpolateOptions{MaxDepth: 3})
	if got.Text != "{{v4}}" {
		t.Fatalf("text = %q, want the token at the depth bound", got.Text)
	}
	if len(got.Unresolved) != 1 || got.Unresolved[0] != "v4" {
		t.Fatalf("unresolved = %v", got.Unresolved)
	}

	// With enough depth the same set resolves to the end of the chain.
	got = set.Interpolate("{{v0}}", InterpolateOptions{MaxDepth: 60})
	if got.Text != "end" {
		t.Fatalf("deep chain = %q", got.Text)
	}
}

func TestInterpolateDefaultDepthIsBounded(t *testing.T) {
	set := NewSet()
	variables := make([]collection.Variable, 0, 40)
	for i := 0; i < 40; i++ {
		variables = append(variables, collection.Variable{
			Name: "v" + strconv.Itoa(i), Value: "{{v" + strconv.Itoa(i+1) + "}}",
			Enabled: true, Kind: collection.VarString,
		})
	}
	set.Load(ScopeRequest, variables)

	// The default bound is ten expansions, so v11 is where it stops.
	got := set.Interpolate("{{v0}}", InterpolateOptions{})
	if got.Text != "{{v11}}" {
		t.Fatalf("text = %q, want the token at the default bound", got.Text)
	}
	if len(got.Unresolved) != 1 || got.Unresolved[0] != "v11" {
		t.Fatalf("unresolved = %v", got.Unresolved)
	}

	// A depth beyond the safety clamp is clamped rather than honoured.
	got = set.Interpolate("{{v0}}", InterpolateOptions{MaxDepth: 1_000_000})
	if !strings.Contains(got.Text, "{{") {
		t.Fatalf("an enormous MaxDepth reached the end: %q", got.Text)
	}
}

func TestInterpolateNamespaces(t *testing.T) {
	set := NewSet()
	set.SetProcessEnv(map[string]string{"TOKEN": "env-token"})
	set.Load(ScopeOAuth2, []collection.Variable{
		{Name: "$oauth2.creds.access_token", Value: "oauth-token", Enabled: true, Kind: collection.VarString},
	})
	set.PutText(ScopePrompt, "User name", "Ada")
	set.PutText(ScopeRequest, "TOKEN", "request-token")

	tests := []struct {
		name string
		text string
		want string
	}{
		{"process.env", "Bearer {{process.env.TOKEN}}", "Bearer env-token"},
		{"oauth2", "{{$oauth2.creds.access_token}}", "oauth-token"},
		{"prompt", "Hello {{?User name}}", "Hello Ada"},
		{"bare name skips process.env", "{{TOKEN}}", "request-token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want {
				t.Fatalf("= %q, want %q", got.Text, tc.want)
			}
		})
	}

	for _, text := range []string{"{{?Missing}}", "{{$oauth2.other.x}}", "{{process.env.NOPE}}"} {
		got := set.Interpolate(text, InterpolateOptions{})
		if got.Text != text || len(got.Unresolved) != 1 {
			t.Fatalf("%s = %q, unresolved %v", text, got.Text, got.Unresolved)
		}
	}
}

func TestInterpolateDynamicHook(t *testing.T) {
	set := newSet()
	opts := InterpolateOptions{Dynamic: func(name string) (string, bool) {
		if name == "uuid" {
			return "fixed-uuid", true
		}
		return "", false
	}}

	got := set.Interpolate("id-{{$uuid}}", opts)
	if got.Text != "id-fixed-uuid" {
		t.Fatalf("text = %q", got.Text)
	}
	got = set.Interpolate("{{$unknown}}", opts)
	if got.Text != "{{$unknown}}" || len(got.Unresolved) != 1 {
		t.Fatalf("= %q, unresolved %v", got.Text, got.Unresolved)
	}

	// Without a resolver every dynamic token stays unresolved.
	got = set.Interpolate("{{$uuid}}", InterpolateOptions{})
	if got.Text != "{{$uuid}}" {
		t.Fatalf("text = %q", got.Text)
	}
}

func TestInterpolateJSONEscaping(t *testing.T) {
	set := newSet(
		"quote", `a"b`,
		"line", "one\ntwo",
		"slash", `c:\tmp`,
		"tab", "a\tb",
		"plain", "simple",
	)
	opts := InterpolateOptions{EscapeJSON: true}

	tests := []struct {
		name string
		text string
		want string
	}{
		{"quote", `{"v":"{{quote}}"}`, `{"v":"a\"b"}`},
		{"newline", `{"v":"{{line}}"}`, `{"v":"one\ntwo"}`},
		{"backslash", `{"v":"{{slash}}"}`, `{"v":"c:\\tmp"}`},
		{"tab", `{"v":"{{tab}}"}`, `{"v":"a\tb"}`},
		{"nothing to escape", `{"v":"{{plain}}"}`, `{"v":"simple"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Interpolate(tc.text, opts)
			if got.Text != tc.want {
				t.Fatalf("= %q, want %q", got.Text, tc.want)
			}
		})
	}

	// Without the option the value is inserted verbatim.
	got := set.Interpolate(`{"v":"{{quote}}"}`, InterpolateOptions{})
	if got.Text != `{"v":"a"b"}` {
		t.Fatalf("unescaped = %q", got.Text)
	}
}

func TestInterpolateEscapesOnlySubstitutedText(t *testing.T) {
	set := newSet("value", `x"y`)

	// The surrounding literal quotes are not part of the substitution.
	got := set.Interpolate(`"{{value}}"`, InterpolateOptions{EscapeJSON: true})
	if got.Text != `"x\"y"` {
		t.Fatalf("= %q", got.Text)
	}
}

func TestInterpolateEscapingAppliesToNestedValues(t *testing.T) {
	set := newSet("inner", `a"b`, "outer", "{{inner}}")

	got := set.Interpolate(`{"v":"{{outer}}"}`, InterpolateOptions{EscapeJSON: true})
	if got.Text != `{"v":"a\"b"}` {
		t.Fatalf("= %q", got.Text)
	}
}

func TestInterpolatePathValues(t *testing.T) {
	set := NewSet()
	set.Load(ScopeRequest, []collection.Variable{{
		Name:    "user",
		Value:   `{"name":"Ada","roles":["admin","dev"]}`,
		Kind:    collection.VarObject,
		Enabled: true,
	}})

	tests := []struct {
		text string
		want string
	}{
		{"{{user.name}}", "Ada"},
		{"role: {{user.roles[1]}}", "role: dev"},
		{"{{user.roles.0}}", "admin"},
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want {
				t.Fatalf("= %q, want %q", got.Text, tc.want)
			}
		})
	}
}

func TestInterpolateWhitespaceIsNotTrimmed(t *testing.T) {
	set := newSet("name", "Ada")

	// Bruno looks the placeholder up verbatim, spaces included.
	got := set.Interpolate("{{ name }}", InterpolateOptions{})
	if got.Text != "{{ name }}" {
		t.Fatalf("text = %q", got.Text)
	}
	if len(got.Unresolved) != 1 || got.Unresolved[0] != " name " {
		t.Fatalf("unresolved = %v", got.Unresolved)
	}
}

func TestInterpolateZeroSet(t *testing.T) {
	var set Set
	got := set.Interpolate("a{{x}}b", InterpolateOptions{})
	if got.Text != "a{{x}}b" || len(got.Unresolved) != 1 {
		t.Fatalf("= %q, unresolved %v", got.Text, got.Unresolved)
	}
}

func TestEscapeJSONString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "plain", input: "plain", want: "plain"},
		{name: "backslash", input: `a\b`, want: `a\\b`},
		{name: "quote", input: `a"b`, want: `a\"b`},
		{name: "newline", input: "a\nb", want: `a\nb`},
		{name: "carriage return", input: "a\rb", want: `a\rb`},
		{name: "tab", input: "a\tb", want: `a\tb`},
		{name: "unicode", input: "unicode é ✓", want: "unicode é ✓"},
		{name: "mixed", input: "\\ \" \n", want: `\\ \" \n`},
		// An invalid UTF-8 byte must survive: ranging over the string would
		// replace it with U+FFFD.
		{name: "invalid utf-8", input: "a\xffb", want: "a\xffb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeJSONString(tc.input); got != tc.want {
				t.Fatalf("escapeJSONString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestInterpolateTokenGrammarMatchesBruno(t *testing.T) {
	set := newSet("a", "A", "{a", "BRACED")

	tests := []struct {
		name       string
		text       string
		want       string
		unresolved []string
	}{
		// Bruno's pattern is {{([^}]+)}}: a third closing brace is literal.
		{name: "extra closing brace", text: "{{a}}}", want: "A}"},
		// A name may start with a brace, so this names "{a".
		{name: "leading brace", text: "{{{a}}}", want: "BRACED}"},
		{name: "space in name", text: "{{a }}", want: "{{a }}", unresolved: []string{"a "}},
		{name: "brace inside name", text: "{{a{b}}", want: "{{a{b}}", unresolved: []string{"a{b"}},
		{name: "empty prompt", text: "{{?}}", want: "{{?}}", unresolved: []string{"?"}},
		{name: "empty dynamic", text: "{{$}}", want: "{{$}}", unresolved: []string{"$"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Interpolate(tc.text, InterpolateOptions{})
			if got.Text != tc.want {
				t.Fatalf("= %q, want %q", got.Text, tc.want)
			}
			if strings.Join(got.Unresolved, ",") != strings.Join(tc.unresolved, ",") {
				t.Fatalf("unresolved = %v, want %v", got.Unresolved, tc.unresolved)
			}
		})
	}
}

func TestInterpolateMasksSecrets(t *testing.T) {
	set := NewSet()
	set.Load(ScopeEnvironment, []collection.Variable{
		{Name: "apiKey", Enabled: true, Secret: true, Kind: collection.VarString},
	})
	set.PutText(ScopeRequest, "plain", "visible")

	got := set.Interpolate("key={{apiKey}}", InterpolateOptions{MaskSecrets: true})
	if got.Text != "key="+MaskedValue {
		t.Fatalf("text = %q", got.Text)
	}
	if !got.Secret {
		t.Fatal("Result.Secret must be set when a secret took part")
	}

	// Without masking the value is inserted (and reported as secret).
	got = set.Interpolate("{{apiKey}}", InterpolateOptions{})
	if got.Text != "" || !got.Secret {
		t.Fatalf("= %q secret=%v", got.Text, got.Secret)
	}

	// A non-secret result does not claim otherwise.
	got = set.Interpolate("{{plain}}", InterpolateOptions{MaskSecrets: true})
	if got.Secret || got.Text != "visible" {
		t.Fatalf("= %q secret=%v", got.Text, got.Secret)
	}
}

func TestInterpolateEscapedWholeTokenIsAString(t *testing.T) {
	set := NewSet()
	set.Load(ScopeRequest, []collection.Variable{
		{Name: "count", Value: "3", Kind: collection.VarNumber, Enabled: true},
		{Name: "obj", Value: `{"a":1}`, Kind: collection.VarObject, Enabled: true},
	})

	// Escaped text is a string in a JSON context, whatever the value held.
	got := set.Interpolate("{{count}}", InterpolateOptions{EscapeJSON: true})
	if got.Text != "3" || got.Kind != collection.VarString {
		t.Fatalf("= %q (%v)", got.Text, got.Kind)
	}
	got = set.Interpolate("{{obj}}", InterpolateOptions{EscapeJSON: true})
	if got.Text != `{\"a\":1}` || got.Kind != collection.VarString {
		t.Fatalf("= %q (%v)", got.Text, got.Kind)
	}
}

func TestInterpolateDynamicIsMemoised(t *testing.T) {
	set := newSet()
	var calls []string
	opts := InterpolateOptions{Dynamic: func(name string) (string, bool) {
		calls = append(calls, name)
		return "v" + name, true
	}}

	got := set.Interpolate("{{$uuid}} {{$uuid}} {{$rand}}", opts)
	if got.Text != "vuuid vuuid vrand" {
		t.Fatalf("text = %q", got.Text)
	}
	if strings.Join(calls, ",") != "uuid,rand" {
		t.Fatalf("resolver calls = %v, want one per distinct name", calls)
	}

	// A miss is memoised too, so a repeated unknown name is only asked once.
	calls = nil
	miss := InterpolateOptions{Dynamic: func(name string) (string, bool) {
		calls = append(calls, name)
		return "", false
	}}
	set.Interpolate("{{$x}}{{$x}}", miss)
	if len(calls) != 1 {
		t.Fatalf("misses = %v, want a single call", calls)
	}
}

func BenchmarkInterpolate(b *testing.B) {
	set := newSet(
		"host", "{{scheme}}://{{domain}}",
		"scheme", "https",
		"domain", "example.com",
		"token", "abc",
	)
	text := "https://{{host}}/v1/users?token={{token}}&missing={{nope}}"
	b.ReportAllocs()
	for b.Loop() {
		set.Interpolate(text, InterpolateOptions{})
	}
}
