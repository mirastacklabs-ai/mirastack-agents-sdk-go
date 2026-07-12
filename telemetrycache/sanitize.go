package telemetrycache

import "strings"

// SanitizePromQL trims surrounding whitespace and common trailing punctuation
// introduced by LLM formatting artifacts.
func SanitizePromQL(q string) string {
	return strings.Trim(strings.TrimSpace(q), " \t\r\n,\"'`;\n")
}
