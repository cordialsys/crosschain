package normalize_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

var spaceReplacer = strings.NewReplacer(
	" ", "_",
	"\n", "_",
	"\r", "_",
)

// Crosschain normalization should be compatible with Treasury resource normalization.

func isValidIdChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '-' || c == '_'
}

func normalizeId(id string) string {
	// remove leading + trailing whitespace
	id = strings.TrimSpace(id)

	// replace every whitespace character with underscore
	id = spaceReplacer.Replace(id)

	// replace every remaining invalid character with dash
	var sb strings.Builder
	for _, c := range id {
		if isValidIdChar(c) {
			sb.WriteRune(c)
		} else {
			// replace with -
			sb.WriteRune('-')
		}
	}

	return sb.String()
}

func TestNormalize(t *testing.T) {
	require.Equal(t, "abc1234", normalizeId("abc1234"))
	require.Equal(t, "abc-1234", normalizeId("abc.1234"))
	require.Equal(t, "abc-1234", normalizeId("abc/1234"))
	require.Equal(t, "abc-1234", normalizeId("abc🙊1234"))
	require.Equal(t, "abc----1234", normalizeId("abc/**/1234"))
	require.Equal(t, "-_abc----1234", normalizeId("    . abc/**/1234"))
}
