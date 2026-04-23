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
