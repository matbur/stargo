package datastar

import "strings"

// attrEscaper mirrors the replacements gomponents applies to attribute values.
var attrEscaper = strings.NewReplacer(
	"\x00", "\uFFFD",
	`"`, "&#34;",
	"'", "&#39;",
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func escapeAttribute(value string) string {
	if !strings.ContainsAny(value, "\x00\"'&<>") {
		return value
	}
	return attrEscaper.Replace(value)
}

// quoteJSString renders value as a single-quoted JavaScript string literal.
func quoteJSString(value string) string {
	if strings.IndexByte(value, 0) >= 0 {
		panic("datastar: value must not contain NUL")
	}
	var b strings.Builder
	b.Grow(len(value) + 2)
	b.WriteByte('\'')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\u2028':
			b.WriteString(`\u2028`)
		case '\u2029':
			b.WriteString(`\u2029`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
