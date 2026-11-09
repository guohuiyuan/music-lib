//go:build windows

package spotify

import "testing"

func TestSpotifyWindowsProxyForScheme(t *testing.T) {
	tests := []struct {
		server string
		scheme string
		want   string
	}{
		{
			server: "127.0.0.1:7897",
			scheme: "https",
			want:   "127.0.0.1:7897",
		},
		{
			server: "http=127.0.0.1:7890;https=127.0.0.1:7891",
			scheme: "https",
			want:   "127.0.0.1:7891",
		},
		{
			server: "http=127.0.0.1:7890;ftp=127.0.0.1:7892",
			scheme: "https",
			want:   "127.0.0.1:7890",
		},
		{
			server: "https=proxy.example:8080",
			scheme: "http",
			want:   "",
		},
	}
	for _, tt := range tests {
		if got := spotifyWindowsProxyForScheme(tt.server, tt.scheme); got != tt.want {
			t.Fatalf(
				"spotifyWindowsProxyForScheme(%q, %q) = %q, want %q",
				tt.server,
				tt.scheme,
				got,
				tt.want,
			)
		}
	}
}
