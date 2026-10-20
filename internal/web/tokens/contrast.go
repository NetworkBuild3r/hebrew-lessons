package tokens

import (
	"fmt"
	"math"
	"strconv"
)

// ParseHex reads #rrggbb or #rrggbbaa. Alpha is 1 for #rrggbb.
func ParseHex(s string) (r, g, b, a float64, err error) {
	if !hexColor.MatchString(s) {
		return 0, 0, 0, 0, fmt.Errorf("tokens: %q is not lowercase #rrggbb or #rrggbbaa", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if len(s) == 7 {
		return float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255, 1, nil
	}
	return float64(v>>24&0xff) / 255, float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255, nil
}

func linear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// Luminance is the WCAG 2 relative luminance of an opaque #rrggbb colour.
func Luminance(hex string) (float64, error) {
	r, g, b, a, err := ParseHex(hex)
	if err != nil {
		return 0, err
	}
	if a != 1 {
		return 0, fmt.Errorf("tokens: %s has alpha; contrast needs an opaque colour", hex)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b), nil
}

// Contrast is the WCAG 2 contrast ratio between two opaque colours.
func Contrast(fg, bg string) (float64, error) {
	lf, err := Luminance(fg)
	if err != nil {
		return 0, err
	}
	lb, err := Luminance(bg)
	if err != nil {
		return 0, err
	}
	hi, lo := math.Max(lf, lb), math.Min(lf, lb)
	return (hi + 0.05) / (lo + 0.05), nil
}
