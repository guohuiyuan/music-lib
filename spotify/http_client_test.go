package spotify

import "testing"

func TestSpotifyParseProxyURL(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "127.0.0.1:7897", want: "http://127.0.0.1:7897"},
		{raw: "http://proxy.example:8080", want: "http://proxy.example:8080"},
		{raw: "socks5://user:pass@proxy.example:1080", want: "socks5://user:pass@proxy.example:1080"},
		{raw: "", want: ""},
	}
	for _, tt := range tests {
		got := spotifyParseProxyURL(tt.raw)
		if tt.want == "" {
			if got != nil {
				t.Fatalf("spotifyParseProxyURL(%q) = %#v, want nil", tt.raw, got)
			}
			continue
		}
		if got == nil || got.String() != tt.want {
			t.Fatalf("spotifyParseProxyURL(%q) = %#v, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestSpotifyProxyBypassed(t *testing.T) {
	tests := []struct {
		host  string
		lists []string
		want  bool
	}{
		{host: "localhost", want: true},
		{host: "127.0.0.1", want: true},
		{host: "open.spotify.com", lists: []string{"*.spotify.com"}, want: true},
		{host: "open.spotify.com", lists: []string{"spotify.com"}, want: true},
		{host: "api.partner.example", lists: []string{"localhost;127.*;*.example"}, want: true},
		{host: "open.spotify.com", lists: []string{"localhost;127.*"}, want: false},
	}
	for _, tt := range tests {
		if got := spotifyProxyBypassed(tt.host, tt.lists...); got != tt.want {
			t.Fatalf(
				"spotifyProxyBypassed(%q, %v) = %v, want %v",
				tt.host,
				tt.lists,
				got,
				tt.want,
			)
		}
	}
}
