package vars

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"

	"apigo/internal/collection"
)

// jsonValue is a decoded JSON value that remembers object member order.
//
// The standard library's map-based decoding loses two things that matter when a
// variable holding an object is interpolated into a body: member order (Marshal
// sorts keys) and the original number spelling. Bruno serialises with
// JSON.stringify, which keeps insertion order, so a round trip through this
// tree keeps an object looking the way the user wrote it.
type jsonValue struct {
	kind jsonKind
	text string
	arr  []jsonValue
	obj  []jsonMember
}

type jsonKind uint8

const (
	jsonNull jsonKind = iota
	jsonBool
	jsonNumber
	jsonString
	jsonArray
	jsonObject
)

type jsonMember struct {
	key   string
	value jsonValue
}

// errNotJSON means a variable declared as an object does not hold JSON. The
// caller treats the path as unresolved, which leaves `{{name}}` in place.
var errNotJSON = errors.New("vars: value is not JSON")

// decodeJSON parses a JSON document into an order-preserving tree.
func decodeJSON(text string) (jsonValue, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()

	value, err := decodeJSONValue(decoder)
	if err != nil {
		return jsonValue{}, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		// Trailing content: `{"a":1} {"b":2}` is not a value we can traverse.
		return jsonValue{}, errNotJSON
	}
	return value, nil
}

func decodeJSONValue(decoder *json.Decoder) (jsonValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return jsonValue{}, errNotJSON
	}
	return decodeJSONToken(decoder, token)
}

func decodeJSONToken(decoder *json.Decoder, token json.Token) (jsonValue, error) {
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			obj := make([]jsonMember, 0, 8)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return jsonValue{}, errNotJSON
				}
				key, ok := keyToken.(string)
				if !ok {
					return jsonValue{}, errNotJSON
				}
				memberValue, err := decodeJSONValue(decoder)
				if err != nil {
					return jsonValue{}, err
				}
				obj = append(obj, jsonMember{key: key, value: memberValue})
			}
			if _, err := decoder.Token(); err != nil {
				return jsonValue{}, errNotJSON
			}
			return jsonValue{kind: jsonObject, obj: obj}, nil
		case '[':
			arr := make([]jsonValue, 0, 8)
			for decoder.More() {
				item, err := decodeJSONValue(decoder)
				if err != nil {
					return jsonValue{}, err
				}
				arr = append(arr, item)
			}
			if _, err := decoder.Token(); err != nil {
				return jsonValue{}, errNotJSON
			}
			return jsonValue{kind: jsonArray, arr: arr}, nil
		}
		return jsonValue{}, errNotJSON
	case string:
		return jsonValue{kind: jsonString, text: value}, nil
	case bool:
		return jsonValue{kind: jsonBool, text: strconv.FormatBool(value)}, nil
	case json.Number:
		return jsonValue{kind: jsonNumber, text: value.String()}, nil
	case nil:
		return jsonValue{kind: jsonNull}, nil
	}
	return jsonValue{}, errNotJSON
}

// encode renders the value as compact JSON, keeping member order and the
// original number spelling.
func (v jsonValue) encode(b *strings.Builder) {
	switch v.kind {
	case jsonNull:
		b.WriteString("null")
	case jsonBool, jsonNumber:
		b.WriteString(v.text)
	case jsonString:
		// Marshal handles quoting and escaping; it escapes <, > and & as \u003c
		// and friends, which parses back to the same string.
		blob, err := json.Marshal(v.text)
		if err != nil {
			b.WriteString("null")
			return
		}
		b.Write(blob)
	case jsonArray:
		b.WriteByte('[')
		for i, item := range v.arr {
			if i > 0 {
				b.WriteByte(',')
			}
			item.encode(b)
		}
		b.WriteByte(']')
	case jsonObject:
		b.WriteByte('{')
		for i, member := range v.obj {
			if i > 0 {
				b.WriteByte(',')
			}
			key, err := json.Marshal(member.key)
			if err != nil {
				b.WriteString(`""`)
			} else {
				b.Write(key)
			}
			b.WriteByte(':')
			member.value.encode(b)
		}
		b.WriteByte('}')
	}
}

// String returns the compact JSON form.
func (v jsonValue) String() string {
	var b strings.Builder
	v.encode(&b)
	return b.String()
}

// value turns a decoded leaf into a typed variable value. Objects and arrays
// keep their JSON text and the object kind.
func (v jsonValue) value() (collection.VarKind, string) {
	switch v.kind {
	case jsonString:
		return collection.VarString, v.text
	case jsonBool:
		return collection.VarBoolean, v.text
	case jsonNumber:
		return collection.VarNumber, formatJSONNumber(v.text)
	case jsonNull:
		// JSON.stringify(null) is "null", and that is what Bruno writes out.
		return collection.VarString, "null"
	default:
		return collection.VarObject, v.String()
	}
}

// formatJSONNumber normalises a number the way JavaScript's String() does for
// the values people actually store: `36.0` becomes `36`, `1e3` becomes `1000`.
//
// Outside [1e-6, 1e21) Go writes an exponent with a two-digit exponent (`1e-07`)
// where JavaScript writes `1e-7`; the JSON is equivalent, the spelling differs.
func formatJSONNumber(text string) string {
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return strconv.FormatInt(integer, 10)
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
		return text
	}
	abs := math.Abs(number)
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		return strconv.FormatFloat(number, 'g', -1, 64)
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}
