package spotify

import (
	"encoding/json"
	"testing"
)

func TestSpotifyEmbedReleaseDate(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "legacy string", raw: `"2026-01-02"`, want: "2026-01-02"},
		{name: "object with iso string", raw: `{"isoString":"2026-10-15T00:00:00Z"}`, want: "2026-10-15T00:00:00Z"},
		{name: "object with year", raw: `{"year":2025}`, want: "2025"},
		{name: "null", raw: `null`, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value spotifyEmbedReleaseDate
			if err := json.Unmarshal([]byte(test.raw), &value); err != nil {
				t.Fatalf("unmarshal release date: %v", err)
			}
			if got := value.String(); got != test.want {
				t.Fatalf("release date = %q, want %q", got, test.want)
			}
		})
	}
}
