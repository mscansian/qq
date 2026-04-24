package client

import "unicode/utf8"

// stripControlBytes removes C0 (0x00–0x1F) control bytes and DEL (0x7F),
// preserving TAB and LF. It also drops any rune in the C1 control range
// (U+0080–U+009F), whether it arrived as a stray byte or as a valid
// two-byte UTF-8 encoding: on a terminal honoring 8-bit controls, U+009B
// (CSI), U+009D (OSC), and friends are as dangerous as their 7-bit
// equivalents.
func stripControlBytes(b []byte) []byte {
	out := b[:0]
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			// invalid byte or truncated sequence — drop one byte
			i++
			continue
		}
		if r >= 0x80 && r <= 0x9F {
			i += size
			continue
		}
		if size == 1 {
			c := byte(r)
			if c == '\t' || c == '\n' || (c >= 0x20 && c != 0x7F) {
				out = append(out, c)
			}
			i++
			continue
		}
		out = append(out, b[i:i+size]...)
		i += size
	}
	return out
}
