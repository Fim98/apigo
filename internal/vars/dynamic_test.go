package vars

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"apigo/internal/collection"
)

// brunoDynamicNames is the table from Bruno 2.0.0's
// packages/bruno-common/src/utils/faker-functions.ts. It is the contract: every
// name here must resolve, and no other name may.
var brunoDynamicNames = []string{
	"guid", "timestamp", "isoTimestamp", "randomUUID", "randomNanoId",
	"randomAlphaNumeric", "randomBoolean", "randomInt", "randomColor",
	"randomHexColor", "randomAbbreviation", "randomIP", "randomIPV4", "randomIPV6",
	"randomMACAddress", "randomPassword", "randomLocale", "randomUserAgent",
	"randomProtocol", "randomSemver",
	"randomFirstName", "randomLastName", "randomFullName", "randomNamePrefix",
	"randomNameSuffix", "randomJobArea", "randomJobDescriptor", "randomJobTitle",
	"randomJobType", "randomPhoneNumber", "randomPhoneNumberExt",
	"randomCity", "randomStreetName", "randomStreetAddress", "randomCountry",
	"randomCountryCode", "randomLatitude", "randomLongitude",
	"randomAvatarImage", "randomImageUrl", "randomAbstractImage",
	"randomAnimalsImage", "randomBusinessImage", "randomCatsImage",
	"randomCityImage", "randomFoodImage", "randomNightlifeImage",
	"randomFashionImage", "randomPeopleImage", "randomNatureImage",
	"randomSportsImage", "randomTransportImage", "randomImageDataUri",
	"randomBankAccount", "randomBankAccountName", "randomCreditCardMask",
	"randomBankAccountBic", "randomBankAccountIban", "randomTransactionType",
	"randomCurrencyCode", "randomCurrencyName", "randomCurrencySymbol",
	"randomBitcoin",
	"randomCompanyName", "randomCompanySuffix", "randomBs", "randomBsAdjective",
	"randomBsBuzz", "randomBsNoun", "randomCatchPhrase",
	"randomCatchPhraseAdjective", "randomCatchPhraseDescriptor",
	"randomCatchPhraseNoun",
	"randomDatabaseColumn", "randomDatabaseType", "randomDatabaseCollation",
	"randomDatabaseEngine",
	"randomDateFuture", "randomDatePast", "randomDateRecent", "randomWeekday",
	"randomMonth",
	"randomDomainName", "randomDomainSuffix", "randomDomainWord", "randomEmail",
	"randomExampleEmail", "randomUserName", "randomUrl",
	"randomFileName", "randomFileType", "randomFileExt", "randomCommonFileName",
	"randomCommonFileType", "randomCommonFileExt", "randomFilePath",
	"randomDirectoryPath", "randomMimeType",
	"randomPrice", "randomProduct", "randomProductAdjective",
	"randomProductMaterial", "randomProductName", "randomDepartment",
	"randomNoun", "randomVerb", "randomIngverb", "randomAdjective", "randomWord",
	"randomWords", "randomPhrase",
	"randomLoremWord", "randomLoremWords", "randomLoremSentence",
	"randomLoremSentences", "randomLoremParagraph", "randomLoremParagraphs",
	"randomLoremText", "randomLoremSlug", "randomLoremLines",
}

// ipv4Octet matches 0-255 without leading zeros, so a generator that drifts past
// the octet range fails here.
const ipv4Octet = `(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)`

// shapeRules pins the value shape of the names where the shape is the point.
var shapeRules = map[string]*regexp.Regexp{
	"guid":                  regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`),
	"randomUUID":            regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`),
	"timestamp":             regexp.MustCompile(`^\d{10}$`),
	"isoTimestamp":          regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`),
	"randomNanoId":          regexp.MustCompile(`^[A-Za-z0-9_-]{21}$`),
	"randomAlphaNumeric":    regexp.MustCompile(`^[A-Za-z0-9]$`),
	"randomBoolean":         regexp.MustCompile(`^(true|false)$`),
	"randomInt":             regexp.MustCompile(`^\d{1,4}$`),
	"randomSemver":          regexp.MustCompile(`^\d+\.\d+\.\d+$`),
	"randomHexColor":        regexp.MustCompile(`^#[0-9a-f]{6}$`),
	"randomIP":              regexp.MustCompile(`^` + ipv4Octet + `(\.` + ipv4Octet + `){3}$`),
	"randomIPV4":            regexp.MustCompile(`^` + ipv4Octet + `(\.` + ipv4Octet + `){3}$`),
	"randomIPV6":            regexp.MustCompile(`^([0-9a-f]{4}:){7}[0-9a-f]{4}$`),
	"randomMACAddress":      regexp.MustCompile(`^([0-9a-f]{2}:){5}[0-9a-f]{2}$`),
	"randomPassword":        regexp.MustCompile(`^[A-Za-z0-9]{15}$`),
	"randomLatitude":        regexp.MustCompile(`^-?\d{1,2}\.\d{4}$`),
	"randomLongitude":       regexp.MustCompile(`^-?\d{1,3}\.\d{4}$`),
	"randomPhoneNumber":     regexp.MustCompile(`^\(\d{3}\) \d{3}-\d{4}$`),
	"randomPhoneNumberExt":  regexp.MustCompile(`^\(\d{3}\) \d{3}-\d{4} x\d{3}$`),
	"randomEmail":           regexp.MustCompile(`^[a-z.]+@[a-z]+\.[a-z]+$`),
	"randomExampleEmail":    regexp.MustCompile(`^[a-z.]+@example\.(com|org|net)$`),
	"randomUrl":             regexp.MustCompile(`^https://[a-z]+\.[a-z]+$`),
	"randomPrice":           regexp.MustCompile(`^\d{1,3}\.\d{2}$`),
	"randomImageDataUri":    regexp.MustCompile(`^data:image/svg\+xml;base64,[A-Za-z0-9+/]+=*$`),
	"randomAvatarImage":     regexp.MustCompile(`^https://avatars\.githubusercontent\.com/u/\d+$`),
	"randomImageUrl":        regexp.MustCompile(`^https://picsum\.photos/seed/[A-Za-z0-9]{8}/640/480$`),
	"randomBankAccount":     regexp.MustCompile(`^\d{8}$`),
	"randomBankAccountIban": regexp.MustCompile(`^DE\d{20}$`),
	"randomBankAccountBic":  regexp.MustCompile(`^[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}$`),
	"randomCreditCardMask":  regexp.MustCompile(`^DE\d{2}\*{16}\d{2}$`),
	"randomBitcoin":         regexp.MustCompile(`^(1[1-9A-HJ-NP-Za-km-z]{33}|bc1[qpzry9x8gf2tvdw0s3jn54khce6mua7l]{39})$`),
	"randomMimeType":        regexp.MustCompile(`^[a-z]+/[a-z0-9.+-]+$`),
	"randomFilePath":        regexp.MustCompile(`^/home/[a-z]+/[a-z_-]+/[a-z_-]+/[a-z_-]+\.[a-z0-9]+$`),
	"randomDirectoryPath":   regexp.MustCompile(`^/home/[a-z]+/[a-z_-]+$`),
	"randomFileExt":         regexp.MustCompile(`^[a-z0-9]+$`),
	"randomCommonFileExt":   regexp.MustCompile(`^[a-z0-9]+$`),
}

// imageCategoriesByName maps the per-category image variables to their URL.
var imageCategoriesByName = map[string]string{
	"randomAbstractImage":  "abstract",
	"randomAnimalsImage":   "animals",
	"randomBusinessImage":  "business",
	"randomCatsImage":      "cats",
	"randomCityImage":      "city",
	"randomFoodImage":      "food",
	"randomNightlifeImage": "nightlife",
	"randomFashionImage":   "fashion",
	"randomPeopleImage":    "people",
	"randomNatureImage":    "nature",
	"randomSportsImage":    "sports",
	"randomTransportImage": "transport",
}

// multilineNames are the lorem values that legitimately contain a line break.
var multilineNames = map[string]bool{
	"randomLoremSentences":  true,
	"randomLoremParagraph":  true,
	"randomLoremParagraphs": true,
	"randomLoremText":       true,
	"randomLoremLines":      true,
}

func TestDynamicNamesMatchBruno(t *testing.T) {
	got := DynamicNames()
	if len(got) != len(brunoDynamicNames) {
		t.Fatalf("table has %d names, Bruno has %d", len(got), len(brunoDynamicNames))
	}
	present := make(map[string]bool, len(got))
	for _, name := range got {
		present[name] = true
	}
	for _, want := range brunoDynamicNames {
		if !present[want] {
			t.Errorf("missing dynamic name %q", want)
		}
	}
	for name := range present {
		if !containsString(brunoDynamicNames, name) {
			t.Errorf("dynamic name %q is not in Bruno's table", name)
		}
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestDynamicValuesHaveTheRightShape(t *testing.T) {
	dynamic := NewDynamic(42)

	for _, name := range brunoDynamicNames {
		t.Run(name, func(t *testing.T) {
			value, ok := dynamic.Lookup(name)
			if !ok {
				t.Fatalf("%s did not resolve", name)
			}
			if value == "" {
				t.Fatalf("%s is empty", name)
			}
			if strings.Contains(value, "\n") && !multilineNames[name] {
				t.Fatalf("%s contains a line break: %q", name, value)
			}
			if rule, ok := shapeRules[name]; ok && !rule.MatchString(value) {
				t.Fatalf("%s = %q, which does not match %s", name, value, rule)
			}
			if category, ok := imageCategoriesByName[name]; ok {
				pattern := regexp.MustCompile(`^https://loremflickr\.com/640/480/` + category + `\?lock=\d{1,5}$`)
				if !pattern.MatchString(value) {
					t.Fatalf("%s = %q, want a %s image URL with a lock", name, value, category)
				}
			}
		})
	}
}

func TestDynamicUnknownName(t *testing.T) {
	dynamic := NewDynamic(1)
	for _, name := range []string{"", "uuid", "randomNothing", "randomint"} {
		if value, ok := dynamic.Lookup(name); ok {
			t.Fatalf("%q resolved to %q", name, value)
		}
	}
}

func TestDynamicIsDeterministic(t *testing.T) {
	// The clock is fixed, so the time-based names are deterministic too: the
	// whole sequence, in name order, must match between two generators.
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	first := NewDynamicWithClock(7, func() time.Time { return now })
	second := NewDynamicWithClock(7, func() time.Time { return now })

	for _, name := range brunoDynamicNames {
		a, _ := first.Lookup(name)
		b, _ := second.Lookup(name)
		if a != b {
			t.Fatalf("%s differs between two generators with the same seed: %q vs %q", name, a, b)
		}
	}

	// A different seed changes the random names; the clock-based ones may stay.
	other := NewDynamicWithClock(8, func() time.Time { return now })
	changed := 0
	for _, name := range brunoDynamicNames {
		a, _ := NewDynamicWithClock(7, func() time.Time { return now }).Lookup(name)
		b, _ := other.Lookup(name)
		if a != b {
			changed++
		}
	}
	if changed < 10 {
		t.Fatalf("a different seed changed only %d values", changed)
	}
}

func TestDynamicNamesAreSorted(t *testing.T) {
	names := DynamicNames()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("DynamicNames is not sorted: %v", names)
	}
}

func TestDynamicUsesTheInjectedClock(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 30, 15, 0, time.UTC)
	dynamic := NewDynamicWithClock(3, func() time.Time { return now })

	if got, _ := dynamic.Lookup("timestamp"); got != strconv.FormatInt(now.Unix(), 10) {
		t.Fatalf("timestamp = %q", got)
	}
	if got, _ := dynamic.Lookup("isoTimestamp"); got != "2026-10-22T09:30:15.000Z" {
		t.Fatalf("isoTimestamp = %q", got)
	}

	// Future is after now, past is before it, both parse as ISO timestamps.
	for _, tc := range []struct {
		name  string
		after bool
	}{
		{"randomDateFuture", true},
		{"randomDatePast", false},
		{"randomDateRecent", false},
	} {
		value, _ := dynamic.Lookup(tc.name)
		parsed, err := time.Parse("2006-01-02T15:04:05.000Z", value)
		if err != nil {
			t.Fatalf("%s = %q: %v", tc.name, value, err)
		}
		if tc.after && !parsed.After(now) {
			t.Fatalf("%s = %q, want after %v", tc.name, value, now)
		}
		if !tc.after && !parsed.Before(now) {
			t.Fatalf("%s = %q, want before %v", tc.name, value, now)
		}
		if tc.name == "randomDateRecent" && now.Sub(parsed) > 24*time.Hour {
			t.Fatalf("%s = %q, want within the last day", tc.name, value)
		}
	}
}

func TestDynamicUserAgentHasNoFormatVerbs(t *testing.T) {
	dynamic := NewDynamic(5)
	for i := 0; i < 50; i++ {
		value, _ := dynamic.Lookup("randomUserAgent")
		if strings.Contains(value, "%!") {
			t.Fatalf("user agent has an unfilled verb: %q", value)
		}
		if !strings.HasPrefix(value, "Mozilla/5.0") {
			t.Fatalf("user agent = %q", value)
		}
	}
}

func TestInterpolateOAuth2BeatsDynamic(t *testing.T) {
	set := NewSet()
	set.Load(ScopeOAuth2, []collection.Variable{
		{Name: "$oauth2.creds.token", Value: "credential", Enabled: true, Kind: collection.VarString},
	})
	// A resolver that answers every name would be wrong here: an exact
	// `$oauth2...` variable must win.
	opts := InterpolateOptions{Dynamic: func(string) (string, bool) { return "dynamic", true }}

	got := set.Interpolate("{{$oauth2.creds.token}}", opts)
	if got.Text != "credential" {
		t.Fatalf("= %q, want the credential", got.Text)
	}
}

func TestDynamicNilClockFallsBackToWallClock(t *testing.T) {
	dynamic := NewDynamicWithClock(1, nil)
	value, ok := dynamic.Lookup("timestamp")
	if !ok || value == "" {
		t.Fatalf("timestamp = %q, ok %v", value, ok)
	}
}

func TestInterpolateDynamicVariables(t *testing.T) {
	dynamic := NewDynamicWithClock(11, func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	})
	set := newSet()
	opts := InterpolateOptions{Dynamic: dynamic.Lookup}

	got := set.Interpolate("id={{$randomUUID}} at {{$timestamp}}", opts)
	if got.Text == "" || strings.Contains(got.Text, "{{") {
		t.Fatalf("text = %q", got.Text)
	}
	if !strings.Contains(got.Text, "at 1767323045") {
		t.Fatalf("text = %q, want the injected timestamp", got.Text)
	}

	// A repeated token is memoised, so both occurrences are identical.
	got = set.Interpolate("{{$randomInt}}-{{$randomInt}}", opts)
	left, right, _ := strings.Cut(got.Text, "-")
	if left == "" || left != right {
		t.Fatalf("memoisation broke: %q", got.Text)
	}

	// Unknown dynamic names stay unresolved.
	got = set.Interpolate("{{$randomNothing}}", opts)
	if got.Text != "{{$randomNothing}}" || len(got.Unresolved) != 1 {
		t.Fatalf("= %q, unresolved %v", got.Text, got.Unresolved)
	}
}
