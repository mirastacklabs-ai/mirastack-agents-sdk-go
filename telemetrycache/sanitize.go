package telemetrycache

import "strings"

// SanitizePromQL trims surrounding whitespace and common trailing punctuation
// introduced by LLM formatting artifacts.
func SanitizePromQL(q string) string {
	return strings.Trim(strings.TrimSpace(q), " \t\r\n,\"'`;\n")
}

// SanitizeLogsQL trims surrounding whitespace and code-fence artifacts while
// preserving valid quoted LogsQL expressions. Unlike SanitizePromQL, it does
// not strip balanced quote characters from either end of the query.
func SanitizeLogsQL(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	if strings.HasPrefix(q, "```") && strings.HasSuffix(q, "```") {
		inner := strings.TrimSuffix(strings.TrimPrefix(q, "```"), "```")
		inner = strings.TrimSpace(inner)
		if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
			first := strings.TrimSpace(inner[:nl])
			rest := strings.TrimSpace(inner[nl+1:])
			// Treat a single-token first line as a fence language marker.
			if first != "" && !strings.ContainsAny(first, " \t") {
				inner = rest
			}
		}
		q = strings.TrimSpace(inner)
	}
	q = trimTrailingSemicolonOutsideQuotes(q)
	return strings.TrimSpace(q)
}

func trimTrailingSemicolonOutsideQuotes(q string) string {
	end := len(q) - 1
	for end >= 0 {
		switch q[end] {
		case ' ', '\t', '\n', '\r':
			end--
		default:
			goto foundEnd
		}
	}
	return ""

foundEnd:
	if q[end] != ';' {
		return q
	}

	inSingle := false
	inDouble := false
	escaped := false
	for i := 0; i <= end; i++ {
		ch := q[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
	}
	if inSingle || inDouble {
		return q
	}
	return strings.TrimSpace(q[:end])
}
