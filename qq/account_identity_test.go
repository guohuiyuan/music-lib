package qq

import "testing"

func TestAccountUINFromCookie(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		want   int64
	}{
		{name: "QQ uin cookie", cookie: "uin=o12345; p_skey=fake", want: 12345},
		{name: "WeChat musicid cookie", cookie: "musicid=67890; qm_keyst=fake", want: 67890},
		{name: "uin takes precedence", cookie: "musicid=67890; uin=o12345", want: 12345},
		{name: "invalid uin falls back to musicid", cookie: "uin=invalid; musicid=67890", want: 67890},
		{name: "missing identity keeps anonymous value", cookie: "qm_keyst=fake", want: 0},
		{name: "empty cookie keeps anonymous value", cookie: "", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.cookie).accountUIN(); got != tt.want {
				t.Fatalf("accountUIN() = %d, want %d", got, tt.want)
			}
		})
	}
}
