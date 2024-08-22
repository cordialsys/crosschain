package normalize_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

var punctuationReplacer = strings.NewReplacer(
	".", "-",
	":", "-",
	",", "-",
	";", "-",
	"!", "-",
	"?", "-",
)
var spaceReplacer = strings.NewReplacer(
	" ", "_",
	"\n", "_",
	"\r", "_",
)

// Crosschain normalization should be compatible with Treasury resource normalization.

func normalizeResourceId(id string) string {
	// remove leading + trailing whitespace
	id = strings.TrimSpace(id)

	// replace all punctuation with dash
	id = punctuationReplacer.Replace(id)

	// replace all whitespace underscore
	id = spaceReplacer.Replace(id)

	// drop everything else that is not valid
	var sb strings.Builder
	for _, c := range id {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '-' || c == '_' {
			sb.WriteRune(c)
		} else {
			// replace with -
			sb.WriteRune('-')
		}
	}

	return sb.String()
}

func TestNormalize(t *testing.T) {
	require.Equal(t, "abc1234", normalizeResourceId("abc1234"))
	require.Equal(t, "abc-1234", normalizeResourceId("abc.1234"))
	require.Equal(t, "abc-1234", normalizeResourceId("abc/1234"))
	require.Equal(t, "abc-1234", normalizeResourceId("abc🙊1234"))
	require.Equal(t, "abc----1234", normalizeResourceId("abc/**/1234"))
	require.Equal(t, "-_abc----1234", normalizeResourceId("    . abc/**/1234"))
}
