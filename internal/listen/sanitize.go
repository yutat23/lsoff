package listen

import "strings"

// SanitizeDisplay strips terminal escapes and replaces control characters so
// process-controlled strings cannot break a table or hijack the TTY.
// JSON output should keep the original bytes.
func SanitizeDisplay(s string) string {
	if s == "" {
		return s
	}
	s = stripEscapes(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 32 || r == 127 || (r >= 0x80 && r <= 0x9f) {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func displayCell(s string) string {
	return dash(strings.TrimSpace(SanitizeDisplay(s)))
}

func stripEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 >= len(s) {
			break
		}
		switch s[i+1] {
		case '[': // CSI
			i = skipUntil(s, i+2, func(c byte) bool { return c >= 0x40 && c <= 0x7e })
		case ']': // OSC
			i = skipOSC(s, i+2)
		case 'P', 'X', '^', '_': // DCS / SOS / PM / APC
			i = skipST(s, i+2)
		case '\\': // stray ST
			i += 2
		default:
			// ESC Fe or ESC + intermediate
			j := i + 2
			for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2f {
				j++
			}
			if j < len(s) && s[j] >= 0x30 && s[j] <= 0x7e {
				i = j + 1
			} else {
				i = j
			}
		}
	}
	return b.String()
}

func skipUntil(s string, i int, done func(byte) bool) int {
	for i < len(s) {
		c := s[i]
		i++
		if done(c) {
			break
		}
	}
	return i
}

func skipOSC(s string, i int) int {
	for i < len(s) {
		if s[i] == 0x07 { // BEL
			return i + 1
		}
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
		i++
	}
	return i
}

func skipST(s string, i int) int {
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
		i++
	}
	return i
}
