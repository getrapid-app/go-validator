package validator

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestArabicCoversEveryMessage(t *testing.T) {
	for k, en := range englishMessages {
		ar, ok := arabicMessages[k]
		if !ok {
			t.Errorf("arabic: missing %q", k)
			continue
		}
		if a, b := placeholders(en), placeholders(ar); a != b {
			t.Errorf("arabic %q: placeholders %s, want %s", k, b, a)
		}
	}
	for k := range arabicMessages {
		if _, ok := englishMessages[k]; !ok {
			t.Errorf("arabic: unknown key %q", k)
		}
	}
}

var placeholderRE = regexp.MustCompile(`:[a-z_]+`)

func placeholders(s string) string {
	seen := map[string]bool{}
	for _, p := range placeholderRE.FindAllString(s, -1) {
		seen[p] = true
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}
