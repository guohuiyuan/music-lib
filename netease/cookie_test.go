package netease

import "testing"

func TestWithClientIdentity(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		want   string
	}{
		{
			name:   "empty cookie stays empty",
			cookie: "",
			want:   "",
		},
		{
			name:   "blank cookie stays empty",
			cookie: " \t ",
			want:   "",
		},
		{
			name:   "adds missing client fields",
			cookie: "MUSIC_U=abc; __csrf=def",
			want:   "MUSIC_U=abc; __csrf=def; os=pc; appver=8.9.70",
		},
		{
			name:   "keeps user supplied values",
			cookie: "MUSIC_U=abc; os=android; appver=9.1.20",
			want:   "MUSIC_U=abc; os=android; appver=9.1.20",
		},
		{
			name:   "only fills the missing field",
			cookie: "MUSIC_U=abc; os=pc",
			want:   "MUSIC_U=abc; os=pc; appver=8.9.70",
		},
		{
			name:   "fills blank values",
			cookie: "MUSIC_U=abc; os=pc; appver=",
			want:   "MUSIC_U=abc; os=pc; appver=8.9.70",
		},
		{
			name:   "replaces blank values without duplicate keys",
			cookie: "MUSIC_U=abc; appver=; os=",
			want:   "MUSIC_U=abc; appver=8.9.70; os=pc",
		},
		{
			name:   "keeps values containing equals signs",
			cookie: "MUSIC_U=abc==; os=pc",
			want:   "MUSIC_U=abc==; os=pc; appver=8.9.70",
		},
		{
			name:   "matches field names case insensitively",
			cookie: "MUSIC_U=abc; OS=pc; AppVer=9.1.0",
			want:   "MUSIC_U=abc; OS=pc; AppVer=9.1.0",
		},
		{
			name:   "does not confuse similar field names",
			cookie: "MUSIC_U=abc; osver=10; appver_extra=1",
			want:   "MUSIC_U=abc; osver=10; appver_extra=1; os=pc; appver=8.9.70",
		},
		{
			name:   "trims trailing separators before appending",
			cookie: " MUSIC_U=abc;  ",
			want:   "MUSIC_U=abc; os=pc; appver=8.9.70",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := withClientIdentity(test.cookie)
			if got != test.want {
				t.Fatalf("withClientIdentity(%q) = %q, want %q", test.cookie, got, test.want)
			}
			if again := withClientIdentity(got); again != got {
				t.Fatalf("withClientIdentity is not idempotent: %q -> %q", got, again)
			}
		})
	}
}

func TestNewNormalizesNeteaseCookie(t *testing.T) {
	if got := New("MUSIC_U=abc").cookie; got != "MUSIC_U=abc; os=pc; appver=8.9.70" {
		t.Fatalf("New cookie = %q, want client identity appended", got)
	}
	if got := New("").cookie; got != "" {
		t.Fatalf("New(\"\") cookie = %q, want empty", got)
	}
	custom := "MUSIC_U=abc; os=android; appver=9.1.20"
	if got := New(custom).cookie; got != custom {
		t.Fatalf("New(custom) cookie = %q, want %q unchanged", got, custom)
	}
}
