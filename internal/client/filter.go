package client

// stripControlBytes removes C0 (0x00–0x1F) control bytes and DEL (0x7F),
// preserving TAB and LF. The C1 range (0x80–0x9F) is handled implicitly:
// those bytes only appear legitimately as UTF-8 continuation bytes, which
// this walker keeps when part of a valid multi-byte sequence. Stray C1
// bytes outside a sequence are dropped.
//
// We walk UTF-8-aware because a naive byte-strip of the C1 range would
// mangle any multi-byte rune whose continuation byte happens to fall in
// 0x80–0x9F.
func stripControlBytes(b []byte) []byte {
	out := b[:0]
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c < 0x80:
			if c == '\t' || c == '\n' || (c >= 0x20 && c != 0x7F) {
				out = append(out, c)
			}
			i++
		case c < 0xC2:
			// stray continuation byte or overlong start — drop
			i++
		case c < 0xE0 && i+1 < len(b) && isCont(b[i+1]):
			out = append(out, b[i:i+2]...)
			i += 2
		case c < 0xF0 && i+2 < len(b) && isCont(b[i+1]) && isCont(b[i+2]):
			out = append(out, b[i:i+3]...)
			i += 3
		case c < 0xF5 && i+3 < len(b) && isCont(b[i+1]) && isCont(b[i+2]) && isCont(b[i+3]):
			out = append(out, b[i:i+4]...)
			i += 4
		default:
			// invalid UTF-8 leading byte or truncated sequence — drop one byte
			i++
		}
	}
	return out
}

func isCont(c byte) bool { return c >= 0x80 && c <= 0xBF }
