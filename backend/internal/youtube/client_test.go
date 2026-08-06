package youtube

import "testing"

func TestParseISODuration(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"PT45S":    45,
		"PT2M3S":   123,
		"PT1H2M3S": 3723,
	}
	for input, want := range cases {
		if got := parseISODuration(input); got != want {
			t.Fatalf("parseISODuration(%q) = %d, want %d", input, got, want)
		}
	}
}
