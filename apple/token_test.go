package apple

import (
	"encoding/base64"
	"testing"
)

func TestExtractAppleAppBundlePath(t *testing.T) {
	tests := []struct {
		name string
		page string
		want string
	}{
		{
			name: "current hashed bundle",
			page: `<script type="module" crossorigin src="/assets/index~cc245af156.js"></script>`,
			want: "/assets/index~cc245af156.js",
		},
		{
			name: "legacy bundle",
			page: `<script src="/assets/index-legacy~1234.js"></script>`,
			want: "/assets/index-legacy~1234.js",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractAppleAppBundlePath([]byte(tc.page)); got != tc.want {
				t.Fatalf("extractAppleAppBundlePath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractAppleDeveloperToken(t *testing.T) {
	makeToken := func(header, claims string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(header)) + "." +
			base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".signature"
	}

	other := makeToken(`{"alg":"ES256","kid":"OTHER"}`, `{"iss":"other-service"}`)
	developer := makeToken(
		`{"typ":"JWT","alg":"ES256","kid":"WebPlayKid"}`,
		`{"iss":"AMPWebPlay","exp":1798828534}`,
	)
	bundle := []byte(`const other="` + other + `";const qa="` + developer + `";`)

	if got := extractAppleDeveloperToken(bundle); got != developer {
		t.Fatalf("extractAppleDeveloperToken() = %q, want developer token", got)
	}
}
