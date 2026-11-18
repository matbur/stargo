package datastar

import (
	"fmt"
	"strings"
)

// encodeJSObject renders a minified JavaScript object literal.
//
// Keys are single-quoted string literals, string values are raw JavaScript
// expressions, and nested maps become nested objects. Keys are sorted, so the
// output depends only on the map contents.
func encodeJSObject(values map[string]any) string {
	keys := sortedKeys(values)
	var b strings.Builder
	b.Grow(len(keys) * 16)
	b.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			b.WriteByte(',')
		}
		b.WriteString(quoteJSString(key))
		b.WriteByte(':')
		switch value := values[key].(type) {
		case string:
			b.WriteString(value)
		case map[string]any:
			b.WriteString(encodeJSObject(value))
		default:
			panic(fmt.Sprintf("datastar: JavaScript object value for %q has unsupported type %T", key, value))
		}
	}
	b.WriteByte('}')
	return b.String()
}

// stringValues converts a string map for encodeJSObject.
func stringValues(mapping map[string]string) map[string]any {
	values := make(map[string]any, len(mapping))
	for key, value := range mapping {
		values[key] = value
	}
	return values
}

// Attrs renders a data-attr attribute with the standard data- prefix.
func Attrs(mapping map[string]string) Attribute {
	return Default().Attrs(mapping)
}

// Attrs renders several HTML attributes reactively as one object.
//
// Keys are lowercase kebab-case attribute names. Values are trusted JavaScript
// expressions evaluated in the browser; never pass user-supplied text. Keys
// are sorted, so the rendered bytes do not depend on map order.
func (a Attributes) Attrs(mapping map[string]string) Attribute {
	validateAttributeKeys(mapping)
	return a.named("attr", encodeJSObject(stringValues(mapping)))
}

func validateAttributeKeys(mapping map[string]string) {
	for _, key := range sortedKeys(mapping) {
		validateAttributeKey(key)
	}
}

// Classes renders a data-class attribute with the standard data- prefix.
func Classes(mapping map[string]string) Attribute {
	return Default().Classes(mapping)
}

// Classes renders several reactive CSS class toggles as one object.
//
// Keys are lowercase kebab-case class names. Values are trusted JavaScript
// expressions evaluated in the browser; never pass user-supplied text. Keys
// are sorted, so the rendered bytes do not depend on map order.
func (a Attributes) Classes(mapping map[string]string) Attribute {
	validateAttributeKeys(mapping)
	return a.named("class", encodeJSObject(stringValues(mapping)))
}

// Styles renders a data-style attribute with the standard data- prefix.
func Styles(mapping map[string]string) Attribute {
	return Default().Styles(mapping)
}

// Styles renders several reactive inline style properties as one object.
//
// Keys are lowercase kebab-case property names. Values are trusted JavaScript
// expressions evaluated in the browser; never pass user-supplied text. Keys
// are sorted, so the rendered bytes do not depend on map order.
func (a Attributes) Styles(mapping map[string]string) Attribute {
	validateAttributeKeys(mapping)
	return a.named("style", encodeJSObject(stringValues(mapping)))
}

// Computeds renders a data-computed attribute with the standard data- prefix.
func Computeds(mapping map[string]string) Attribute {
	return Default().Computeds(mapping)
}

// Computeds renders several computed signals as one object.
//
// Each value becomes a zero-argument function returning the expression, and
// dotted names expand into nested objects, so ui.total addresses $ui.total.
// Values are trusted JavaScript; never pass user-supplied text. A name used
// both as a signal and as a path prefix panics, as does an empty segment.
func (a Attributes) Computeds(mapping map[string]string) Attribute {
	functions := make(map[string]any, len(mapping))
	for name, expression := range mapping {
		functions[name] = "() => (" + expression + ")"
	}
	return a.named("computed", encodeJSObject(expandSignalPaths(functions)))
}
