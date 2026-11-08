// Package slug membuat slug URL dari nama (dipakai seeder dan service produk).
package slug

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Make: "D & G" → "d-g", "1000 Bunga" → "1000-bunga".
func Make(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
