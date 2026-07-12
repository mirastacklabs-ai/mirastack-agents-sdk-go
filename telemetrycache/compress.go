package telemetrycache

import (
	"bytes"
	"compress/gzip"
	"io"
)

var gzipMagic = []byte{0x1f, 0x8b}

// CompressValue returns the input as-is when below threshold, and gzip-compresses
// larger payloads. If the payload exceeds MAX_ENTRY_SIZE, ok=false is returned.
func CompressValue(value string) (compressed string, ok bool) {
	if len(value) > maxCacheEntryBytes {
		return "", false
	}
	if len(value) < compressThreshold {
		return value, true
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(value)); err != nil {
		_ = zw.Close()
		return "", false
	}
	if err := zw.Close(); err != nil {
		return "", false
	}
	return buf.String(), true
}

// DecompressValue transparently gunzips values that begin with gzip magic bytes.
func DecompressValue(value string) (string, error) {
	raw := []byte(value)
	if len(raw) < 2 || raw[0] != gzipMagic[0] || raw[1] != gzipMagic[1] {
		return value, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
