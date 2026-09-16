package rawtags

import (
	"testing"

	"github.com/dhowden/tag"
)

func TestExtract_VorbisStyleLowercasedKeys(t *testing.T) {
	raw := map[string]interface{}{
		"title":          "Some Song",
		"hitsyncyear":    "1971",
		"hitsyncexclude": "1",
	}
	res := extract(raw)
	if res.Year == nil || *res.Year != 1971 {
		t.Errorf("Year = %v, want 1971", res.Year)
	}
	if !res.Exclude {
		t.Errorf("Exclude = false, want true")
	}
}

func TestExtract_MP4StyleMixedCaseKeys(t *testing.T) {
	raw := map[string]interface{}{
		"HITSYNCYEAR": "1968",
	}
	res := extract(raw)
	if res.Year == nil || *res.Year != 1968 {
		t.Errorf("Year = %v, want 1968", res.Year)
	}
	if res.Exclude {
		t.Errorf("Exclude = true, want false")
	}
}

func TestExtract_ExcludeOnlyTrueForExactly1(t *testing.T) {
	for _, v := range []string{"0", "true", "yes", ""} {
		raw := map[string]interface{}{"hitsyncexclude": v}
		if extract(raw).Exclude {
			t.Errorf("Exclude = true for value %q, want false", v)
		}
	}
}

func TestExtract_ID3v2TXXXFrames(t *testing.T) {
	raw := map[string]interface{}{
		"TXXX":   &tag.Comm{Description: "HitsyncYear", Text: "1955"},
		"TXXX_0": &tag.Comm{Description: "HitsyncExclude", Text: "1"},
		"TXXX_1": &tag.Comm{Description: "SomeOtherTag", Text: "irrelevant"},
	}
	res := extract(raw)
	if res.Year == nil || *res.Year != 1955 {
		t.Errorf("Year = %v, want 1955", res.Year)
	}
	if !res.Exclude {
		t.Errorf("Exclude = false, want true")
	}
}

func TestExtract_AbsentTagsYieldZeroValue(t *testing.T) {
	res := extract(map[string]interface{}{"title": "No custom tags here"})
	if res.Year != nil {
		t.Errorf("Year = %v, want nil", res.Year)
	}
	if res.Exclude {
		t.Errorf("Exclude = true, want false")
	}
}

func TestExtract_UnparsableYearIgnored(t *testing.T) {
	res := extract(map[string]interface{}{"hitsyncyear": "not-a-year"})
	if res.Year != nil {
		t.Errorf("Year = %v, want nil for unparsable value", res.Year)
	}
}
