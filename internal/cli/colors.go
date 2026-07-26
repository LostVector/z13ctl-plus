package cli

// colors.go — named color lookup and display for --color / --color2 flags.
//
// Names are matched case-insensitively. Hex strings (with or without #) are
// passed through to ParseColor unchanged.

import (
	"fmt"
	"strings"
)

type namedColor struct {
	Name string
	Hex  string // 6-digit lowercase hex, no #
}

// namedColorList is the ordered list of supported color names, arranged in
// spectral order so the swatch output reads as a natural gradient.
var namedColorList = []namedColor{
	{"red", "ff0000"},
	{"crimson", "dc143c"},
	{"orangered", "ff4500"},
	{"coral", "ff7f50"},
	{"orange", "ff8000"},
	{"gold", "ffd700"},
	{"yellow", "ffff00"},
	{"chartreuse", "7fff00"},
	{"green", "00ff00"},
	{"springgreen", "00ff7f"},
	{"aquamarine", "7fffd4"},
	{"teal", "008080"},
	{"cyan", "00ffff"},
	{"deepskyblue", "00bfff"},
	{"dodgerblue", "1e90ff"},
	{"royalblue", "4169e1"},
	{"blue", "0000ff"},
	{"navy", "000080"},
	{"indigo", "4b0082"},
	{"blueviolet", "8a2be2"},
	{"purple", "800080"},
	{"magenta", "ff00ff"},
	{"deeppink", "ff1493"},
	{"hotpink", "ff69b4"},
	{"violet", "ee82ee"},
	{"turquoise", "40e0d0"},
	{"brown", "a52a2a"},
	{"white", "ffffff"},
}

// NamedColors is the lookup map built from namedColorList at init time.
var NamedColors = map[string]string{}

func init() {
	for _, c := range namedColorList {
		NamedColors[c.Name] = c.Hex
	}
}

// ResolveColor returns the 6-digit hex string for a color name or hex input.
// If s is a known name, its hex value is returned. Otherwise s is returned
// as-is (to be validated by ParseColor).
func ResolveColor(s string) string {
	if hex, ok := NamedColors[strings.ToLower(s)]; ok {
		return hex
	}
	return s
}

// ColorDisplay returns a human-readable label for a color string.
// If s is a known name it returns the name; otherwise it returns "#XXXXXX".
func ColorDisplay(s string) string {
	lower := strings.ToLower(s)
	if _, ok := NamedColors[lower]; ok {
		return lower
	}
	hex := strings.ToUpper(strings.TrimPrefix(s, "#"))
	return "#" + hex
}

// PrintColorList prints all named colors with ANSI true-color swatches.
func PrintColorList() {
	fmt.Println("\nSupported color names:")
	for _, c := range namedColorList {
		r, g, b, _ := ParseColor(c.Hex)
		fmt.Printf("  \033[48;2;%d;%d;%dm      \033[0m  %-12s  #%s\n", r, g, b, c.Name, strings.ToUpper(c.Hex))
	}
	fmt.Println()
}
