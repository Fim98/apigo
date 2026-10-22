package vars

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Dynamic generates the values behind `{{$name}}` tokens.
//
// Bruno builds this table on faker-js; apigo implements the same names with its
// own generators and small word lists, so a value has the same shape (a valid
// IPv4, an ISO timestamp, a plausible name) but not faker's exact vocabulary or
// distribution. The names are the contract; the values are compatible in kind.
//
// Give the same seed and clock to get the same sequence, which is what the tests
// and the runner's deterministic mode rely on. A Dynamic is not safe for
// concurrent use, like Set itself.
type Dynamic struct {
	rnd *rand.Rand
	now func() time.Time
}

// NewDynamic returns a generator seeded with seed and reading the wall clock.
func NewDynamic(seed int64) *Dynamic {
	return NewDynamicWithClock(seed, time.Now)
}

// NewDynamicWithClock is NewDynamic with an injectable clock, so timestamp and
// date variables are testable.
func NewDynamicWithClock(seed int64, now func() time.Time) *Dynamic {
	if now == nil {
		now = time.Now
	}
	return &Dynamic{rnd: rand.New(rand.NewSource(seed)), now: now}
}

// Lookup resolves one dynamic name. An unknown name reports false, which leaves
// the token unresolved.
func (d *Dynamic) Lookup(name string) (string, bool) {
	generator, ok := dynamicGenerators[name]
	if !ok {
		return "", false
	}
	return generator(d), true
}

// DynamicNames returns every supported dynamic name, sorted.
func DynamicNames() []string {
	names := make([]string, 0, len(dynamicGenerators))
	for name := range dynamicGenerators {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// pick returns one element of list.
func (d *Dynamic) pick(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[d.rnd.Intn(len(list))]
}

// intBetween returns a value in [min, max].
func (d *Dynamic) intBetween(min, max int) int {
	if max <= min {
		return min
	}
	return min + d.rnd.Intn(max-min+1)
}

// digits returns n random decimal digits.
func (d *Dynamic) digits(n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte(byte('0' + d.rnd.Intn(10)))
	}
	return b.String()
}

// alphanumeric returns n characters from [a-zA-Z0-9], the mixed casing faker
// uses for alphanumeric, passwords and nanoids.
func (d *Dynamic) alphanumeric(n int) string {
	return d.fromAlphabet(n, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

// upperAlphanumeric returns n characters from [A-Z0-9].
func (d *Dynamic) upperAlphanumeric(n int) string {
	return d.fromAlphabet(n, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

// upperLetters returns n uppercase letters.
func (d *Dynamic) upperLetters(n int) string {
	return d.fromAlphabet(n, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

// fromAlphabet draws n random characters from alphabet.
func (d *Dynamic) fromAlphabet(n int, alphabet string) string {
	if n <= 0 || alphabet == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[d.rnd.Intn(len(alphabet))])
	}
	return b.String()
}

// hex returns n lowercase hex characters.
func (d *Dynamic) hex(n int) string {
	return d.fromAlphabet(n, "0123456789abcdef")
}

// uuid returns a random (version 4) UUID.
func (d *Dynamic) uuid() string {
	var raw [16]byte
	_, _ = d.rnd.Read(raw[:])
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

// semver returns a major.minor.patch string.
func (d *Dynamic) semver() string {
	return fmt.Sprintf("%d.%d.%d", d.intBetween(0, 9), d.intBetween(0, 20), d.intBetween(0, 99))
}

// ipv4 returns a dotted quad.
func (d *Dynamic) ipv4() string {
	octets := make([]string, 4)
	for i := range octets {
		octets[i] = strconv.Itoa(d.intBetween(1, 254))
	}
	return strings.Join(octets, ".")
}

// ipv6 returns eight groups of four hex digits.
func (d *Dynamic) ipv6() string {
	groups := make([]string, 8)
	for i := range groups {
		groups[i] = d.hex(4)
	}
	return strings.Join(groups, ":")
}

// macAddress returns a colon-separated MAC address.
func (d *Dynamic) macAddress() string {
	parts := make([]string, 6)
	for i := range parts {
		parts[i] = d.hex(2)
	}
	return strings.Join(parts, ":")
}

// underscored joins the words of text with underscores, so the result is safe as
// one path segment.
func underscored(text string) string {
	return strings.Join(strings.Fields(text), "_")
}

// sentence builds a capitalized sentence of n lorem words.
func (d *Dynamic) sentence(n int) string {
	if n <= 0 || len(loremWords) == 0 {
		return ""
	}
	words := make([]string, n)
	for i := range words {
		words[i] = d.pick(loremWords)
	}
	first := []rune(words[0])
	words[0] = strings.ToUpper(string(first[0])) + string(first[1:])
	return strings.Join(words, " ") + "."
}

// paragraph builds sentences separated by spaces, the way faker does.
func (d *Dynamic) paragraph(sentences int) string {
	parts := make([]string, sentences)
	for i := range parts {
		parts[i] = d.sentence(d.intBetween(4, 8))
	}
	return strings.Join(parts, " ")
}

// paragraphs builds paragraphs separated by a blank line.
func (d *Dynamic) paragraphs(count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = d.paragraph(d.intBetween(3, 5))
	}
	return strings.Join(parts, "\n\n")
}

// words builds n space-separated lorem words.
func (d *Dynamic) words(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = d.pick(loremWords)
	}
	return strings.Join(parts, " ")
}

// imageURL builds a LoremFlickr placeholder URL. The lock parameter makes each
// generated URL point at a different image, which is the point of a random one.
func imageURL(d *Dynamic, category string) string {
	url := "https://loremflickr.com/640/480"
	if category != "" {
		url += "/" + category
	}
	return url + "?lock=" + strconv.Itoa(d.intBetween(1, 99999))
}

// dynamicGenerators is the authoritative name table. Adding a name here is all
// it takes for `{{$name}}` to resolve.
var dynamicGenerators = map[string]func(d *Dynamic) string{
	// Identifiers and time.
	"guid":       func(d *Dynamic) string { return d.uuid() },
	"randomUUID": func(d *Dynamic) string { return d.uuid() },
	"timestamp":  func(d *Dynamic) string { return strconv.FormatInt(d.now().Unix(), 10) },
	"isoTimestamp": func(d *Dynamic) string {
		return d.now().UTC().Format("2006-01-02T15:04:05.000Z")
	},
	"randomNanoId":       func(d *Dynamic) string { return d.fromAlphabet(21, nanoIDAlphabet) },
	"randomAlphaNumeric": func(d *Dynamic) string { return d.alphanumeric(1) },
	"randomBoolean":      func(d *Dynamic) string { return strconv.FormatBool(d.rnd.Intn(2) == 1) },
	"randomInt":          func(d *Dynamic) string { return strconv.Itoa(d.intBetween(0, 1000)) },
	"randomSemver":       func(d *Dynamic) string { return d.semver() },
	"randomColor":        func(d *Dynamic) string { return d.pick(colorNames) },
	"randomHexColor":     func(d *Dynamic) string { return "#" + d.hex(6) },
	"randomLocale":       func(d *Dynamic) string { return d.pick(countryCodes) },
	"randomProtocol":     func(d *Dynamic) string { return d.pick([]string{"http", "https"}) },
	"randomAbbreviation": func(d *Dynamic) string { return d.pick(hackerAbbreviations) },
	"randomIP":           func(d *Dynamic) string { return d.ipv4() },
	"randomIPV4":         func(d *Dynamic) string { return d.ipv4() },
	"randomIPV6":         func(d *Dynamic) string { return d.ipv6() },
	"randomMACAddress":   func(d *Dynamic) string { return d.macAddress() },
	"randomPassword":     func(d *Dynamic) string { return d.alphanumeric(15) },
	"randomUserAgent":    func(d *Dynamic) string { return userAgent(d) },
	"randomAvatarImage":  func(d *Dynamic) string { return avatarURL(d) },
	"randomImageUrl": func(d *Dynamic) string {
		return "https://picsum.photos/seed/" + d.alphanumeric(8) + "/640/480"
	},
	"randomImageDataUri":   func(d *Dynamic) string { return imageDataURI(d) },
	"randomAbstractImage":  func(d *Dynamic) string { return imageURL(d, "abstract") },
	"randomAnimalsImage":   func(d *Dynamic) string { return imageURL(d, "animals") },
	"randomBusinessImage":  func(d *Dynamic) string { return imageURL(d, "business") },
	"randomCatsImage":      func(d *Dynamic) string { return imageURL(d, "cats") },
	"randomCityImage":      func(d *Dynamic) string { return imageURL(d, "city") },
	"randomFoodImage":      func(d *Dynamic) string { return imageURL(d, "food") },
	"randomNightlifeImage": func(d *Dynamic) string { return imageURL(d, "nightlife") },
	"randomFashionImage":   func(d *Dynamic) string { return imageURL(d, "fashion") },
	"randomPeopleImage":    func(d *Dynamic) string { return imageURL(d, "people") },
	"randomNatureImage":    func(d *Dynamic) string { return imageURL(d, "nature") },
	"randomSportsImage":    func(d *Dynamic) string { return imageURL(d, "sports") },
	"randomTransportImage": func(d *Dynamic) string { return imageURL(d, "transport") },

	// Person.
	"randomFirstName":     func(d *Dynamic) string { return d.pick(firstNames) },
	"randomLastName":      func(d *Dynamic) string { return d.pick(lastNames) },
	"randomFullName":      func(d *Dynamic) string { return d.pick(firstNames) + " " + d.pick(lastNames) },
	"randomNamePrefix":    func(d *Dynamic) string { return d.pick(namePrefixes) },
	"randomNameSuffix":    func(d *Dynamic) string { return d.pick(nameSuffixes) },
	"randomJobArea":       func(d *Dynamic) string { return d.pick(jobAreas) },
	"randomJobDescriptor": func(d *Dynamic) string { return d.pick(jobDescriptors) },
	"randomJobTitle": func(d *Dynamic) string {
		return d.pick(jobDescriptors) + " " + d.pick(jobTypes)
	},
	"randomJobType": func(d *Dynamic) string { return d.pick(jobTypes) },
	"randomPhoneNumber": func(d *Dynamic) string {
		return fmt.Sprintf("(%s) %s-%s", d.digits(3), d.digits(3), d.digits(4))
	},
	"randomPhoneNumberExt": func(d *Dynamic) string {
		return fmt.Sprintf("(%s) %s-%s x%s", d.digits(3), d.digits(3), d.digits(4), d.digits(3))
	},

	// Location.
	"randomCity": func(d *Dynamic) string { return d.pick(cities) },
	"randomStreetName": func(d *Dynamic) string {
		return d.pick(streetNames) + " " + d.pick(streetSuffixes)
	},
	"randomStreetAddress": func(d *Dynamic) string {
		return fmt.Sprintf("%d %s %s", d.intBetween(1, 9999), d.pick(streetNames), d.pick(streetSuffixes))
	},
	"randomCountry":     func(d *Dynamic) string { return d.pick(countries) },
	"randomCountryCode": func(d *Dynamic) string { return d.pick(countryCodes) },
	"randomLatitude": func(d *Dynamic) string {
		return strconv.FormatFloat(float64(d.intBetween(-900000, 900000))/10000, 'f', 4, 64)
	},
	"randomLongitude": func(d *Dynamic) string {
		return strconv.FormatFloat(float64(d.intBetween(-1800000, 1800000))/10000, 'f', 4, 64)
	},

	// Internet.
	"randomDomainName": func(d *Dynamic) string {
		return d.pick(domainWords) + "." + d.pick(domainSuffixes)
	},
	"randomDomainSuffix": func(d *Dynamic) string { return d.pick(domainSuffixes) },
	"randomDomainWord":   func(d *Dynamic) string { return d.pick(domainWords) },
	"randomEmail": func(d *Dynamic) string {
		return emailLocal(d) + "@" + d.pick(domainWords) + "." + d.pick(domainSuffixes)
	},
	"randomExampleEmail": func(d *Dynamic) string {
		return emailLocal(d) + "@example." + d.pick([]string{"com", "org", "net"})
	},
	"randomUserName": func(d *Dynamic) string {
		return strings.ToLower(d.pick(firstNames)) + "_" + strings.ToLower(d.pick(lastNames)) + d.digits(2)
	},
	"randomUrl": func(d *Dynamic) string {
		return "https://" + d.pick(domainWords) + "." + d.pick(domainSuffixes)
	},

	// Company.
	"randomCompanyName": func(d *Dynamic) string {
		return d.pick(lastNames) + " " + d.pick(companySuffixes)
	},
	// Bruno points this at faker.company.name(), so a full company name is the
	// parity behaviour even though the name says "suffix".
	"randomCompanySuffix": func(d *Dynamic) string {
		return d.pick(lastNames) + " " + d.pick(companySuffixes)
	},
	"randomCatchPhrase": func(d *Dynamic) string {
		return d.pick(catchPhraseAdjectives) + " " + d.pick(catchPhraseDescriptors) + " " + d.pick(catchPhraseNouns)
	},
	"randomCatchPhraseAdjective":  func(d *Dynamic) string { return d.pick(catchPhraseAdjectives) },
	"randomCatchPhraseDescriptor": func(d *Dynamic) string { return d.pick(catchPhraseDescriptors) },
	"randomCatchPhraseNoun":       func(d *Dynamic) string { return d.pick(catchPhraseNouns) },
	"randomBs": func(d *Dynamic) string {
		return d.pick(buzzVerbs) + " " + d.pick(buzzAdjectives) + " " + d.pick(buzzNouns)
	},
	"randomBsAdjective": func(d *Dynamic) string { return d.pick(buzzAdjectives) },
	"randomBsBuzz":      func(d *Dynamic) string { return d.pick(buzzVerbs) },
	"randomBsNoun":      func(d *Dynamic) string { return d.pick(buzzNouns) },

	// Database.
	"randomDatabaseColumn":    func(d *Dynamic) string { return d.pick(databaseColumns) },
	"randomDatabaseType":      func(d *Dynamic) string { return d.pick(databaseTypes) },
	"randomDatabaseCollation": func(d *Dynamic) string { return d.pick(databaseCollations) },
	"randomDatabaseEngine":    func(d *Dynamic) string { return d.pick(databaseEngines) },

	// Dates.
	"randomDateFuture": func(d *Dynamic) string { return isoAfter(d, time.Duration(d.intBetween(1, 365))*24*time.Hour) },
	"randomDatePast":   func(d *Dynamic) string { return isoAfter(d, -time.Duration(d.intBetween(1, 365))*24*time.Hour) },
	"randomDateRecent": func(d *Dynamic) string {
		// faker's recent() defaults to the last day.
		return isoAfter(d, -time.Duration(d.intBetween(1, 24*60*60))*time.Second)
	},
	"randomWeekday": func(d *Dynamic) string { return d.pick(weekdays) },
	"randomMonth":   func(d *Dynamic) string { return d.pick(months) },

	// Files and system.
	"randomFileName": func(d *Dynamic) string {
		return underscored(d.words(2)) + "." + d.pick(fileExts)
	},
	"randomFileType": func(d *Dynamic) string { return d.pick(fileTypes) },
	"randomFileExt":  func(d *Dynamic) string { return d.pick(fileExts) },
	"randomCommonFileName": func(d *Dynamic) string {
		return underscored(d.words(2)) + "." + d.pick(commonFileExts)
	},
	"randomCommonFileType": func(d *Dynamic) string { return d.pick(fileTypes) },
	"randomCommonFileExt":  func(d *Dynamic) string { return d.pick(commonFileExts) },
	"randomFilePath": func(d *Dynamic) string {
		return "/home/" + strings.ToLower(d.pick(firstNames)) + "/" + underscored(d.words(1)) + "/" +
			underscored(d.words(1)) + "/" + underscored(d.words(2)) + "." + d.pick(fileExts)
	},
	"randomDirectoryPath": func(d *Dynamic) string {
		return "/home/" + strings.ToLower(d.pick(firstNames)) + "/" + underscored(d.words(1))
	},
	"randomMimeType": func(d *Dynamic) string { return d.pick(mimeTypes) },

	// Finance.
	"randomBankAccount":     func(d *Dynamic) string { return d.digits(8) },
	"randomBankAccountName": func(d *Dynamic) string { return d.pick(accountNames) },
	"randomBankAccountBic": func(d *Dynamic) string {
		return d.upperLetters(4) + d.pick(countryCodes) + d.upperAlphanumeric(2)
	},
	"randomBankAccountIban": func(d *Dynamic) string { return "DE" + d.digits(20) },
	// Bruno masks an IBAN: the first four and last two characters stay visible.
	"randomCreditCardMask": func(d *Dynamic) string {
		iban := "DE" + d.digits(20)
		return iban[:4] + strings.Repeat("*", len(iban)-6) + iban[len(iban)-2:]
	},
	"randomTransactionType": func(d *Dynamic) string { return d.pick(transactionTypes) },
	"randomCurrencyCode":    func(d *Dynamic) string { return d.pick(currencyCodes) },
	"randomCurrencyName":    func(d *Dynamic) string { return d.pick(currencyNames) },
	"randomCurrencySymbol":  func(d *Dynamic) string { return d.pick(currencySymbols) },
	"randomBitcoin": func(d *Dynamic) string {
		if d.rnd.Intn(2) == 0 {
			return "1" + d.fromAlphabet(33, bitcoinAlphabet)
		}
		return "bc1" + d.fromAlphabet(39, bech32Alphabet)
	},

	// Commerce.
	"randomPrice": func(d *Dynamic) string {
		return fmt.Sprintf("%d.%02d", d.intBetween(1, 999), d.intBetween(0, 99))
	},
	"randomProduct":          func(d *Dynamic) string { return d.pick(products) },
	"randomProductAdjective": func(d *Dynamic) string { return d.pick(productAdjectives) },
	"randomProductMaterial":  func(d *Dynamic) string { return d.pick(productMaterials) },
	"randomProductName": func(d *Dynamic) string {
		return d.pick(productAdjectives) + " " + d.pick(productMaterials) + " " + d.pick(products)
	},
	"randomDepartment": func(d *Dynamic) string { return d.pick(departments) },

	// Hacker and lorem.
	"randomNoun":      func(d *Dynamic) string { return d.pick(hackerNouns) },
	"randomVerb":      func(d *Dynamic) string { return d.pick(hackerVerbs) },
	"randomIngverb":   func(d *Dynamic) string { return d.pick(hackerIngverbs) },
	"randomAdjective": func(d *Dynamic) string { return d.pick(hackerAdjectives) },
	"randomWord":      func(d *Dynamic) string { return d.pick(hackerNouns) },
	"randomWords":     func(d *Dynamic) string { return d.words(3) },
	"randomPhrase": func(d *Dynamic) string {
		return "We need to " + d.pick(hackerVerbs) + " the " + d.pick(hackerAdjectives) + " " + d.pick(hackerNouns) + "!"
	},
	"randomLoremWord":       func(d *Dynamic) string { return d.pick(loremWords) },
	"randomLoremWords":      func(d *Dynamic) string { return d.words(3) },
	"randomLoremSentence":   func(d *Dynamic) string { return d.sentence(d.intBetween(3, 6)) },
	"randomLoremSentences":  func(d *Dynamic) string { return d.paragraph(2) },
	"randomLoremParagraph":  func(d *Dynamic) string { return d.paragraph(d.intBetween(3, 5)) },
	"randomLoremParagraphs": func(d *Dynamic) string { return d.paragraphs(3) },
	"randomLoremText":       func(d *Dynamic) string { return d.paragraphs(1) },
	"randomLoremSlug":       func(d *Dynamic) string { return strings.ReplaceAll(d.words(3), " ", "-") },
	"randomLoremLines": func(d *Dynamic) string {
		return strings.Join([]string{d.sentence(4), d.sentence(5), d.sentence(4)}, "\n")
	},
}

// isoAfter formats now+offset as an ISO timestamp, which is what Bruno's
// randomDate* variables return.
func isoAfter(d *Dynamic, offset time.Duration) string {
	return d.now().Add(offset).UTC().Format("2006-01-02T15:04:05.000Z")
}

// userAgent builds a browser user agent from a template, filling in versions.
func userAgent(d *Dynamic) string {
	template := d.pick(userAgentTemplates)
	args := make([]any, strings.Count(template, "%d"))
	for i := range args {
		args[i] = d.intBetween(90, 130)
	}
	return fmt.Sprintf(template, args...)
}

// avatarURL returns a deterministic-looking avatar URL.
func avatarURL(d *Dynamic) string {
	return "https://avatars.githubusercontent.com/u/" + strconv.Itoa(d.intBetween(1, 999999))
}

// imageDataURI returns a base64 SVG data URI, which is what faker's dataUri()
// produces. The rectangle's colour is random, so two calls differ.
func imageDataURI(d *Dynamic) string {
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="#%s"/></svg>`,
		d.hex(6),
	)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// emailLocal builds a lowercase local part that stays inside the allowed
// characters even when a surname carries a hyphen.
func emailLocal(d *Dynamic) string {
	raw := strings.ToLower(d.pick(firstNames) + "." + d.pick(lastNames))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || r == '.' {
			return r
		}
		return -1
	}, raw)
}
