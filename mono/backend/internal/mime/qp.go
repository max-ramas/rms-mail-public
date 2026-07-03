// Package mime provides MIME content decoding utilities shared across the backend.
package mime

import "strings"

// DecodeQuotedPrintable decodes RFC 2045 quoted-printable encoded text.
// Removes soft line breaks (=CRLF/=LF) and decodes =XX hex sequences.
func DecodeQuotedPrintable(s string) string {
	s = strings.ReplaceAll(s, "=\r\n", "")
	s = strings.ReplaceAll(s, "=\n", "")

	var result strings.Builder
	result.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '=' && i+2 < len(s) {
			high := unhex(s[i+1])
			low := unhex(s[i+2])
			if high >= 0 && low >= 0 {
				result.WriteByte(byte(high<<4 | low))
				i += 2
				continue
			}
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

func unhex(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'A' <= c && c <= 'F':
		return int(c - 'A' + 10)
	case 'a' <= c && c <= 'f':
		return int(c - 'a' + 10)
	default:
		return -1
	}
}
