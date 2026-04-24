package client

import "testing"

func TestStripControlBytes(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
	}{
		"plain ascii": {in: "hello world", want: "hello world"},
		"keeps tab and newline": {
			in:   "a\tb\nc",
			want: "a\tb\nc",
		},
		"strips ansi escape": {
			in:   "\x1b[31mred\x1b[0m",
			want: "[31mred[0m",
		},
		"strips bell and backspace": {
			in:   "hi\x07\x08there",
			want: "hithere",
		},
		"strips DEL": {
			in:   "a\x7fb",
			want: "ab",
		},
		"strips C1 control rune encoded as UTF-8": {
			// U+009D (OSC) = 0xC2 0x9D. The byte pair is valid UTF-8, but
			// 8-bit terminals interpret it as an OSC introducer.
			in:   "a\xc2\x9d]52;c;AAAA\x07b",
			want: "a]52;c;AAAAb",
		},
		"strips C1 CSI rune encoded as UTF-8": {
			// U+009B (CSI) = 0xC2 0x9B.
			in:   "x\xc2\x9b31my",
			want: "x31my",
		},
		"preserves utf-8 multibyte with high continuation bytes": {
			// é = 0xC3 0xA9, ñ = 0xC3 0xB1 — continuation bytes in C1 range
			in:   "café ñandú",
			want: "café ñandú",
		},
		"preserves 3-byte and 4-byte utf-8": {
			in:   "日本 🎉",
			want: "日本 🎉",
		},
		"empty": {in: "", want: ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// stripControlBytes mutates the underlying slice, so each call
			// needs its own buffer.
			got := string(stripControlBytes([]byte(tc.in)))
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
