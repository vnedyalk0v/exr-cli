// Package strutil holds tiny shared string helpers (one place, no deps).
package strutil

// Truncate cuts s to at most n runes, appending "…" when shortened.
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// AppendLine joins body and line with a newline (empty body → line only).
func AppendLine(body, line string) string {
	if body == "" {
		return line
	}
	return body + "\n" + line
}
