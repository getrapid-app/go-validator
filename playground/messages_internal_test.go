package playground

import (
	"regexp"
	"slices"
	"testing"
)

var placeholder = regexp.MustCompile(`:[a-z_]+`)

func TestMessagesCoverEveryTag(t *testing.T) {
	for tag, info := range tags {
		if _, ok := english[tag]; !ok {
			t.Errorf("no English message for %s", tag)
		}
		if info.typed {
			for _, k := range []string{"string", "numeric", "array"} {
				if _, ok := english[tag+"."+k]; !ok {
					t.Errorf("no English message for %s.%s", tag, k)
				}
			}
		}
	}
	for key, en := range english {
		ar, ok := arabic[key]
		if !ok {
			t.Errorf("no Arabic message for %s", key)
			continue
		}
		pe, pa := placeholder.FindAllString(en, -1), placeholder.FindAllString(ar, -1)
		slices.Sort(pe)
		slices.Sort(pa)
		if !slices.Equal(pe, pa) {
			t.Errorf("%s: English uses %v, Arabic %v", key, pe, pa)
		}
	}
	for key := range arabic {
		if _, ok := english[key]; !ok {
			t.Errorf("Arabic message %s has no English one", key)
		}
	}
}
