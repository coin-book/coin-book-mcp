package coinbook

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFormatMatchesAPIVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/format-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Minor     int64  `json:"minor"`
		Currency  string `json:"currency"`
		Formatted string `json:"formatted"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		if got := Format(Amount{Minor: v.Minor, Currency: v.Currency}); got != v.Formatted {
			t.Errorf("Format(%d %s) = %q, want %q", v.Minor, v.Currency, got, v.Formatted)
		}
	}
}
