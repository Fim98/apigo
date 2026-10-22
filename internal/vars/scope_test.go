package vars

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apigo/internal/collection"
)

// enabled returns one enabled variable, which is how a parsed file looks.
func enabled(name, value string) []collection.Variable {
	return []collection.Variable{{Name: name, Value: value, Enabled: true, Kind: collection.VarString}}
}

func TestPrecedenceAdjacentLayers(t *testing.T) {
	pairs := [][2]Scope{
		{ScopeGlobal, ScopeCollection},
		{ScopeCollection, ScopeEnvironment},
		{ScopeEnvironment, ScopeFolder},
		{ScopeFolder, ScopeRequest},
		{ScopeRequest, ScopeOAuth2},
		{ScopeOAuth2, ScopeRuntime},
		{ScopeRuntime, ScopePrompt},
	}
	for _, pair := range pairs {
		lower, higher := pair[0], pair[1]
		set := NewSet()
		set.Load(lower, enabled("X", lower.String()))
		set.Load(higher, enabled("X", higher.String()))

		got, ok := set.Lookup("X")
		if !ok {
			t.Fatalf("%v over %v: X missing", higher, lower)
		}
		if got.Text != higher.String() || got.Scope != higher {
			t.Fatalf("%v over %v: got %q from %v", higher, lower, got.Text, got.Scope)
		}
	}
}

func TestPrecedenceFullStack(t *testing.T) {
	order := Precedence()
	if len(order) != 8 {
		t.Fatalf("precedence has %d layers, want 8", len(order))
	}

	set := NewSet()
	for _, scope := range order {
		set.Load(scope, enabled("X", scope.String()))
	}

	// Peel the winner off one layer at a time; the next layer must take over.
	for i := len(order) - 1; i >= 0; i-- {
		got, ok := set.Lookup("X")
		if !ok {
			t.Fatalf("layer %v: X missing", order[i])
		}
		if got.Scope != order[i] || got.Text != order[i].String() {
			t.Fatalf("layer %d: got %q from %v, want %v", i, got.Text, got.Scope, order[i])
		}
		delete(set.layers[order[i]], "X")
	}
	if _, ok := set.Lookup("X"); ok {
		t.Fatal("X still resolves after every layer was cleared")
	}
}

func TestLoadSkipsDisabledAndUnnamed(t *testing.T) {
	set := NewSet()
	set.Load(ScopeGlobal, enabled("X", "global"))
	set.Load(ScopeRequest, []collection.Variable{
		{Name: "X", Value: "disabled", Enabled: false},
		{Name: "", Value: "unnamed", Enabled: true},
	})

	got, ok := set.Lookup("X")
	if !ok || got.Text != "global" {
		t.Fatalf("X = %q (%v), want the lower layer to show through", got.Text, ok)
	}
	if got, ok := set.Lookup(""); ok {
		t.Fatalf("empty name resolved to %+v", got)
	}
}

func TestLoadLaterValueWinsWithinALayer(t *testing.T) {
	set := NewSet()
	set.Load(ScopeRequest, []collection.Variable{
		{Name: "X", Value: "first", Enabled: true},
		{Name: "X", Value: "second", Enabled: true},
	})
	if got, _ := set.Lookup("X"); got.Text != "second" {
		t.Fatalf("X = %q, want the later value in the same layer", got.Text)
	}
}

func TestPutIgnoresEnabledFlag(t *testing.T) {
	set := NewSet()
	// A runtime value or prompt answer is not a file entry; the engine decides
	// to set it, so the zero Enabled must not drop it.
	set.Put(ScopeRuntime, collection.Variable{Name: "X", Value: "runtime"})
	if got, ok := set.Lookup("X"); !ok || got.Text != "runtime" {
		t.Fatalf("runtime X = %q (%v)", got.Text, ok)
	}
}

func TestLookupExactNameBeatsPathTraversal(t *testing.T) {
	set := NewSet()
	set.Load(ScopeRequest, []collection.Variable{
		{Name: "user", Value: `{"name":"nested"}`, Kind: collection.VarObject, Enabled: true},
	})
	set.PutText(ScopeCollection, "user.name", "flat")

	got, ok := set.Lookup("user.name")
	if !ok || got.Text != "flat" {
		t.Fatalf("user.name = %q (%v), want the exact name in the lower layer", got.Text, ok)
	}
	if got.Scope != ScopeCollection {
		t.Fatalf("scope = %v, want collection", got.Scope)
	}
}

func TestLookupPaths(t *testing.T) {
	set := NewSet()
	set.Load(ScopeRequest, []collection.Variable{{
		Name:    "user",
		Value:   `{"name":"Ada","age":36.0,"active":true,"address":{"city":"Paris"},"tags":["a","b"],"rows":[{"id":1},{"id":2}],"nothing":null,"inner":{"b":1,"a":2}}`,
		Kind:    collection.VarObject,
		Enabled: true,
	}})
	set.Load(ScopeRequest, []collection.Variable{{
		Name:    "list",
		Value:   `[{"id":1},{"id":2}]`,
		Kind:    collection.VarObject,
		Enabled: true,
	}})

	tests := []struct {
		path string
		text string
		kind collection.VarKind
	}{
		{"user.name", "Ada", collection.VarString},
		// A JSON 36.0 is rendered the way JavaScript's String() renders it.
		{"user.age", "36", collection.VarNumber},
		{"user.active", "true", collection.VarBoolean},
		{"user.address.city", "Paris", collection.VarString},
		{"user.tags[0]", "a", collection.VarString},
		{"user.tags[1]", "b", collection.VarString},
		// lodash `get` accepts a dotted index as well.
		{"user.tags.0", "a", collection.VarString},
		{"user.rows[1].id", "2", collection.VarNumber},
		// Member order survives, so an interpolated object looks as written.
		{"user.inner", `{"b":1,"a":2}`, collection.VarObject},
		{"user.nothing", "null", collection.VarString},
		{"list[1]", `{"id":2}`, collection.VarObject},
		{"list[0].id", "1", collection.VarNumber},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got, ok := set.Lookup(tc.path)
			if !ok {
				t.Fatalf("%s did not resolve", tc.path)
			}
			if got.Text != tc.text || got.Kind != tc.kind {
				t.Fatalf("= %q (%v), want %q (%v)", got.Text, got.Kind, tc.text, tc.kind)
			}
		})
	}

	unresolved := []string{
		"user.missing",
		"user.tags[9]",
		"user.tags[-1]",
		"user.name.first",
		"user[0]",
		"user..name",
		".user",
		"user.",
		"user.tags[x]",
		"user.tags[",
		"user.tags[0]name",
		"user.rows[0].missing",
	}
	for _, path := range unresolved {
		t.Run("unresolved/"+path, func(t *testing.T) {
			if got, ok := set.Lookup(path); ok {
				t.Fatalf("%q resolved to %+v, want unresolved", path, got)
			}
		})
	}
}

func TestLookupOnNonObjectValueStops(t *testing.T) {
	set := NewSet()
	set.PutText(ScopeRequest, "user", "not json")
	if _, ok := set.Lookup("user.name"); ok {
		t.Fatal("a string variable must not be traversed")
	}
}

func TestProcessEnvNamespaceIsSeparate(t *testing.T) {
	set := NewSet()
	set.SetProcessEnv(map[string]string{"TOKEN": "from-env"})

	got, ok := set.Lookup("process.env.TOKEN")
	if !ok || got.Text != "from-env" {
		t.Fatalf("process.env.TOKEN = %q (%v)", got.Text, ok)
	}
	if got.Scope != ScopeProcessEnv {
		t.Fatalf("scope = %v, want process.env", got.Scope)
	}
	if _, ok := set.Lookup("TOKEN"); ok {
		t.Fatal("a process.env name must not resolve as a bare variable")
	}
	if _, ok := set.Lookup("process.env.MISSING"); ok {
		t.Fatal("a missing process.env name resolved")
	}

	// A request variable with the same name is independent of process.env.
	set.PutText(ScopeRequest, "TOKEN", "from-request")
	if got, _ := set.Lookup("TOKEN"); got.Text != "from-request" {
		t.Fatalf("TOKEN = %q", got.Text)
	}
	if got, _ := set.Lookup("process.env.TOKEN"); got.Text != "from-env" {
		t.Fatalf("process.env.TOKEN = %q", got.Text)
	}
}

func TestSetProcessEnvCopiesTheMap(t *testing.T) {
	source := map[string]string{"A": "1"}
	set := NewSet()
	set.SetProcessEnv(source)
	source["A"] = "changed"
	if got, _ := set.Lookup("process.env.A"); got.Text != "1" {
		t.Fatalf("A = %q, want the value captured at SetProcessEnv time", got.Text)
	}
}

func TestOAuth2CredentialsUseTheirFullName(t *testing.T) {
	set := NewSet()
	set.Load(ScopeOAuth2, enabled("$oauth2.creds.access_token", "abc"))
	set.Load(ScopeRequest, enabled("access_token", "request"))

	got, ok := set.Lookup("$oauth2.creds.access_token")
	if !ok || got.Text != "abc" {
		t.Fatalf("oauth2 token = %q (%v)", got.Text, ok)
	}
	if scope, _ := set.ScopeOf("$oauth2.creds.access_token"); scope != ScopeOAuth2 {
		t.Fatalf("scope = %v", scope)
	}
	if _, ok := set.Lookup("$oauth2.other.access_token"); ok {
		t.Fatal("another credential id must not resolve")
	}
}

func TestNamesAndLen(t *testing.T) {
	set := NewSet()
	set.Load(ScopeRequest, []collection.Variable{
		{Name: "b", Value: "1", Enabled: true},
		{Name: "a", Value: "2", Enabled: true},
		{Name: "off", Value: "3", Enabled: false},
	})
	if got := strings.Join(set.Names(ScopeRequest), ","); got != "a,b" {
		t.Fatalf("names = %s", got)
	}
	if got := set.Names(ScopePrompt); len(got) != 0 {
		t.Fatalf("empty layer names = %v", got)
	}
}

// writeCollection builds a `.bru` collection on disk and loads it.
func writeCollection(t *testing.T, files map[string]string) *collection.Collection {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := collection.Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.Warnings) != 0 {
		t.Fatalf("warnings: %v", c.Warnings)
	}
	return c
}

// sampleCollection has a collection var, two folder levels and requests with and
// without their own variables.
func sampleCollection(t *testing.T) *collection.Collection {
	t.Helper()
	return writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"scopes","type":"collection"}`,
		"collection.bru": `meta {
  name: scopes
}

vars:pre-request {
  shared: collection
  collectionOnly: yes
}
`,
		"users/folder.bru": `meta {
  name: Users
}

vars:pre-request {
  shared: users
  tenant: users
}
`,
		"users/admin/folder.bru": `meta {
  name: Admin
}

vars:pre-request {
  shared: admin
}
`,
		"users/list.bru": `meta {
  name: List
  type: http
  seq: 2
}

get {
  url: https://example.com/users
}
`,
		"users/admin/delete.bru": `meta {
  name: Delete
  type: http
  seq: 1
}

delete {
  url: https://example.com/users/:id
}

vars:pre-request {
  shared: request
}
`,
		"environments/Base.bru": `vars {
  region: eu
  token: base-token
}
`,
		"environments/Local.bru": `extends: Base

vars {
  region: local
  token: local-token
}
`,
	})
}

func requestNode(t *testing.T, c *collection.Collection, rel string) *collection.Node {
	t.Helper()
	var found *collection.Node
	if err := c.Walk(func(n *collection.Node) error {
		if n.Request != nil && n.RelPath == rel {
			found = n
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if found == nil {
		t.Fatalf("request %q not found", rel)
	}
	return found
}

func folderNode(t *testing.T, c *collection.Collection, rel string) *collection.Node {
	t.Helper()
	var found *collection.Node
	if err := c.Walk(func(n *collection.Node) error {
		if n.IsFolder() && n.RelPath == rel {
			found = n
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if found == nil {
		t.Fatalf("folder %q not found", rel)
	}
	return found
}

func TestForRequestLayers(t *testing.T) {
	c := sampleCollection(t)
	node := requestNode(t, c, "users/list.bru")

	set, err := ForRequest(c, node, Options{})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}

	// users/folder.bru wins over collection.bru.
	if got, ok := set.Lookup("shared"); !ok || got.Text != "users" {
		t.Fatalf("shared = %q (%v), want the folder value", got.Text, ok)
	}
	if got, ok := set.Lookup("collectionOnly"); !ok || got.Text != "yes" {
		t.Fatalf("collectionOnly = %q (%v)", got.Text, ok)
	}
	if scope, _ := set.ScopeOf("collectionOnly"); scope != ScopeCollection {
		t.Fatalf("collectionOnly scope = %v", scope)
	}
	if scope, _ := set.ScopeOf("shared"); scope != ScopeFolder {
		t.Fatalf("shared scope = %v", scope)
	}
	if names := set.Names(ScopeRequest); len(names) != 0 {
		t.Fatalf("request layer = %v, want no entries for this file", names)
	}
	if names := set.Names(ScopeFolder); len(names) != 2 {
		t.Fatalf("folder layer = %v, want shared and tenant", names)
	}
}

func TestForRequestNearestFolderWins(t *testing.T) {
	c := sampleCollection(t)

	deep, err := ForRequest(c, requestNode(t, c, "users/admin/delete.bru"), Options{})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	// The request's own variable is above both folders.
	if got, _ := deep.Lookup("shared"); got.Text != "request" || got.Scope != ScopeRequest {
		t.Fatalf("deep shared = %q from %v", got.Text, got.Scope)
	}
	// A name only the outer folder sets still resolves.
	if got, _ := deep.Lookup("tenant"); got.Text != "users" || got.Scope != ScopeFolder {
		t.Fatalf("tenant = %q from %v", got.Text, got.Scope)
	}

	middle := folderNode(t, c, "users/admin")
	admin, err := ForRequest(c, middle, Options{})
	if err != nil {
		t.Fatalf("ForRequest folder: %v", err)
	}
	if got, _ := admin.Lookup("shared"); got.Text != "admin" || got.Scope != ScopeFolder {
		t.Fatalf("admin shared = %q from %v", got.Text, got.Scope)
	}
	if names := admin.Names(ScopeRequest); len(names) != 0 {
		t.Fatalf("folder node has a request layer: %v", names)
	}
	// admin's own `shared` plus the outer folder's `tenant`.
	if names := admin.Names(ScopeFolder); len(names) != 2 {
		t.Fatalf("admin folder layer = %v", names)
	}
}

func TestForRequestGlobalBelowCollection(t *testing.T) {
	c := sampleCollection(t)
	set, err := ForRequest(c, requestNode(t, c, "users/list.bru"), Options{
		Global: enabled("shared", "global"),
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, _ := set.Lookup("shared"); got.Text != "users" {
		t.Fatalf("shared = %q, want the folder value above global", got.Text)
	}

	set, err = ForRequest(c, requestNode(t, c, "users/list.bru"), Options{
		Global: enabled("onlyGlobal", "yes"),
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, scope := set.Lookup("onlyGlobal"); !scope || got.Scope != ScopeGlobal {
		t.Fatalf("onlyGlobal = %+v", got)
	}
}

func TestForRequestEnvironment(t *testing.T) {
	c := sampleCollection(t)
	node := requestNode(t, c, "users/list.bru")

	var local *collection.Environment
	for _, env := range c.Environments {
		if env.Name == "Local" {
			local = env
		}
	}
	if local == nil {
		t.Fatal("Local environment not loaded")
	}

	set, err := ForRequest(c, node, Options{Environment: local})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	// The environment's own value wins over the one it extends.
	if got, scope := set.Lookup("region"); !scope || got.Text != "local" || got.Scope != ScopeEnvironment {
		t.Fatalf("region = %q from %v, want local from the environment", got.Text, got.Scope)
	}
	if got, _ := set.Lookup("token"); got.Text != "local-token" {
		t.Fatalf("token = %q", got.Text)
	}

	// --env-var overrides sit above the file within the same layer.
	set, err = ForRequest(c, node, Options{
		Environment:          local,
		EnvironmentOverrides: map[string]string{"region": "override", "added": "yes"},
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, scope := set.Lookup("region"); !scope || got.Text != "override" || got.Scope != ScopeEnvironment {
		t.Fatalf("region = %q from %v", got.Text, got.Scope)
	}
	if got, _ := set.Lookup("added"); got.Text != "yes" {
		t.Fatalf("added = %q", got.Text)
	}

	// No environment selected: the layer is empty.
	set, err = ForRequest(c, node, Options{})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if _, ok := set.Lookup("region"); ok {
		t.Fatal("region resolved without an environment")
	}
}

func TestForRequestRuntimeAndPromptWin(t *testing.T) {
	c := sampleCollection(t)
	node := requestNode(t, c, "users/admin/delete.bru")

	set, err := ForRequest(c, node, Options{
		Runtime: enabled("shared", "runtime"),
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, _ := set.Lookup("shared"); got.Text != "runtime" || got.Scope != ScopeRuntime {
		t.Fatalf("shared = %q from %v", got.Text, got.Scope)
	}

	set, err = ForRequest(c, node, Options{
		Runtime: enabled("shared", "runtime"),
		Prompt:  map[string]string{"shared": "answer"},
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, _ := set.Lookup("shared"); got.Text != "answer" || got.Scope != ScopePrompt {
		t.Fatalf("shared = %q from %v, want the prompt answer", got.Text, got.Scope)
	}
}

func TestForRequestProcessEnv(t *testing.T) {
	c := writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"env","type":"collection"}`,
		".env":       "TOKEN=from-file\n",
		"one.bru": `meta {
  name: One
  type: http
  seq: 1
}

get {
  url: https://example.com
}
`,
	})
	node := requestNode(t, c, "one.bru")

	set, err := ForRequest(c, node, Options{
		ProcessEnv: c.ProcessEnv([]string{"TOKEN=from-os", "HOME=/tmp"}, nil),
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, ok := set.Lookup("process.env.TOKEN"); !ok || got.Text != "from-file" {
		t.Fatalf("process.env.TOKEN = %q (%v)", got.Text, ok)
	}
	if got, ok := set.Lookup("process.env.HOME"); !ok || got.Text != "/tmp" {
		t.Fatalf("process.env.HOME = %q (%v)", got.Text, ok)
	}
	// The dotenv name is not a bare variable.
	if _, ok := set.Lookup("TOKEN"); ok {
		t.Fatal("TOKEN resolved as a bare variable; .env must stay in process.env")
	}
}

func TestForRequestSecretKeepsItsFlag(t *testing.T) {
	c := writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"secrets","type":"collection"}`,
		"collection.bru": `meta {
  name: secrets
}
`,
		"req.bru": `meta {
  name: Req
  type: http
  seq: 1
}

get {
  url: https://example.com
}
`,
		"environments/Prod.bru": `vars:secret [
  apiKey
]
`,
	})
	var prod *collection.Environment
	for _, env := range c.Environments {
		if env.Name == "Prod" {
			prod = env
		}
	}
	if prod == nil {
		t.Fatal("Prod environment not loaded")
	}

	set, err := ForRequest(c, requestNode(t, c, "req.bru"), Options{Environment: prod})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	got, ok := set.Lookup("apiKey")
	if !ok || !got.Secret {
		t.Fatalf("apiKey = %+v, ok %v; a secret declaration must resolve and stay masked", got, ok)
	}
	if got.Text != "" {
		t.Fatalf("apiKey = %q, want an empty value until the keychain is read", got.Text)
	}
}

func TestForRequestEnvironmentCycleIsAnError(t *testing.T) {
	c := writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"cycle","type":"collection"}`,
		"one.bru": `meta {
  name: One
  type: http
  seq: 1
}

get {
  url: https://example.com
}
`,
		"environments/A.bru": `extends: B

vars {
  a: 1
}
`,
		"environments/B.bru": `extends: A

vars {
  b: 2
}
`,
	})
	var a *collection.Environment
	for _, env := range c.Environments {
		if env.Name == "A" {
			a = env
		}
	}
	_, err := ForRequest(c, requestNode(t, c, "one.bru"), Options{Environment: a})
	if err == nil {
		t.Fatal("an extends cycle must be an error")
	}
	if !strings.Contains(err.Error(), "extends itself") {
		t.Fatalf("error = %v", err)
	}
}

func TestForRequestRejectsNilInputs(t *testing.T) {
	c := sampleCollection(t)
	if _, err := ForRequest(nil, nil, Options{}); err == nil {
		t.Fatal("nil collection must be an error")
	}
	if _, err := ForRequest(c, nil, Options{}); err == nil {
		t.Fatal("nil node must be an error")
	}
}

func TestLookupExactNameBeatsTheProcessEnvNamespace(t *testing.T) {
	set := NewSet()
	set.SetProcessEnv(map[string]string{"X": "namespace"})
	set.PutText(ScopeRequest, "process.env.X", "literal")

	// lodash `get` checks a literal key before splitting on dots, so a variable
	// named `process.env.X` wins over the namespace.
	if got, ok := set.Lookup("process.env.X"); !ok || got.Text != "literal" {
		t.Fatalf("process.env.X = %q (%v)", got.Text, ok)
	}
}

func TestProcessEnvEmptyValueIsResolved(t *testing.T) {
	set := NewSet()
	set.SetProcessEnv(map[string]string{"EMPTY": ""})

	got, ok := set.Lookup("process.env.EMPTY")
	if !ok {
		t.Fatal("an empty process.env value must resolve, not fall through")
	}
	if got.Text != "" {
		t.Fatalf("value = %q", got.Text)
	}
	if _, ok := set.Lookup("process.env.MISSING"); ok {
		t.Fatal("a missing process.env name resolved")
	}
}

func TestPrecedenceReturnsACopyWithoutNamespaces(t *testing.T) {
	order := Precedence()
	if len(order) != 8 {
		t.Fatalf("order = %v", order)
	}
	for _, scope := range order {
		if scope == ScopeProcessEnv {
			t.Fatal("process.env must not be part of the precedence order")
		}
	}
	order[0] = ScopePrompt
	if again := Precedence(); again[0] != ScopeGlobal {
		t.Fatalf("Precedence() exposed its backing array: %v", again)
	}
}

func TestZeroValueSetWorks(t *testing.T) {
	var set Set
	set.PutText(ScopeRuntime, "X", "1")
	if got, ok := set.Lookup("X"); !ok || got.Text != "1" {
		t.Fatalf("X = %q (%v) on the zero value", got.Text, ok)
	}
	if _, ok := set.Lookup("process.env.NOPE"); ok {
		t.Fatal("a zero-value set resolved a process.env name")
	}
	if got := set.Names(ScopePrompt); len(got) != 0 {
		t.Fatalf("names = %v", got)
	}
}

func TestPutRejectsNonLayerScopes(t *testing.T) {
	for _, scope := range []Scope{0, ScopeProcessEnv, 99} {
		t.Run(scope.String(), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("Put(%v) did not panic", scope)
				}
			}()
			NewSet().Put(scope, collection.Variable{Name: "X", Value: "1"})
		})
	}
}

func TestForRequestDisabledEnvironmentVariableIsSkipped(t *testing.T) {
	c := writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"disabled","type":"collection"}`,
		"one.bru": `meta {
  name: One
  type: http
  seq: 1
}

get {
  url: https://example.com
}
`,
		"environments/Env.bru": `vars {
  on: yes
  ~off: no
}
`,
	})
	var env *collection.Environment
	for _, e := range c.Environments {
		if e.Name == "Env" {
			env = e
		}
	}
	set, err := ForRequest(c, requestNode(t, c, "one.bru"), Options{Environment: env})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, ok := set.Lookup("on"); !ok || got.Text != "yes" {
		t.Fatalf("on = %q (%v)", got.Text, ok)
	}
	if _, ok := set.Lookup("off"); ok {
		t.Fatal("a disabled environment variable resolved")
	}
}

func TestForRequestOverridesWithoutAnEnvironment(t *testing.T) {
	c := sampleCollection(t)
	set, err := ForRequest(c, requestNode(t, c, "users/list.bru"), Options{
		EnvironmentOverrides: map[string]string{"onlyOverride": "yes", "blank": ""},
	})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	// --env-var is a CLI input; it must not require a selected environment.
	if got, ok := set.Lookup("onlyOverride"); !ok || got.Text != "yes" {
		t.Fatalf("onlyOverride = %q (%v)", got.Text, ok)
	}
	if got, ok := set.Lookup("blank"); !ok || got.Text != "" {
		t.Fatalf("blank = %q (%v), want a resolved empty value", got.Text, ok)
	}
}

func TestForRequestWithoutCollectionDefaults(t *testing.T) {
	c := writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"bare","type":"collection"}`,
		"folder/folder.bru": `meta {
  name: Folder
}

vars:pre-request {
  fromFolder: yes
}
`,
		"folder/one.bru": `meta {
  name: One
  type: http
  seq: 1
}

get {
  url: https://example.com
}
`,
	})
	if c.Defaults != nil {
		t.Fatalf("expected no collection.bru, got %+v", c.Defaults)
	}
	set, err := ForRequest(c, requestNode(t, c, "folder/one.bru"), Options{})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	if got, ok := set.Lookup("fromFolder"); !ok || got.Scope != ScopeFolder {
		t.Fatalf("fromFolder = %q from %v", got.Text, got.Scope)
	}
}

func TestForRequestOwnPlainVariableBeatsInheritedSecret(t *testing.T) {
	c := writeCollection(t, map[string]string{
		"bruno.json": `{"version":"1","name":"secret","type":"collection"}`,
		"one.bru": `meta {
  name: One
  type: http
  seq: 1
}

get {
  url: https://example.com
}
`,
		"environments/Base.bru": `vars:secret [
  token
]
`,
		"environments/Local.bru": `extends: Base

vars {
  token: plain
}
`,
	})
	var local *collection.Environment
	for _, env := range c.Environments {
		if env.Name == "Local" {
			local = env
		}
	}
	set, err := ForRequest(c, requestNode(t, c, "one.bru"), Options{Environment: local})
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}
	got, ok := set.Lookup("token")
	if !ok || got.Text != "plain" || got.Secret {
		t.Fatalf("token = %+v, ok %v; the environment's own plain value must win", got, ok)
	}
}
