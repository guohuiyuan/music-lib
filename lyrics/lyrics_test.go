package lyrics

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseYRCAndConvertVerbatimLRC(t *testing.T) {
	orig := ParseYRC("[1000,1000](1000,500,0)你(1500,500,0)好\n[2500,800](2500,800,0)世界")
	_, ts := ParseLRC("[00:01.00]hello\n[00:02.50]world")
	_, roma := ParseLRC("[00:01.00]ni hao\n[00:02.50]shi jie")

	got := ConvertVerbatimLRC(map[string]string{"ti": "song"}, MultiData{
		"orig": orig,
		"ts":   ts,
		"roma": roma,
	}, []string{"orig", "ts", "roma"})

	for _, want := range []string{
		"[ti:song]",
		"[00:01.00]你[00:01.50]好[00:02.00]",
		"[00:01.00]hello[00:02.00]",
		"[00:01.00]ni hao[00:02.00]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("converted lrc missing %q:\n%s", want, got)
		}
	}
}

func TestParseQRC(t *testing.T) {
	raw := `<Lyric_1 LyricType="1" LyricContent="[ti:T]&#10;[1000,1000]你(1000,500)好(1500,500)"/>`
	tags, data := ParseQRC(raw)
	if tags["ti"] != "T" {
		t.Fatalf("tag ti = %q", tags["ti"])
	}
	if len(data) != 1 || len(data[0].Words) != 2 {
		t.Fatalf("unexpected qrc data: %#v", data)
	}
	if data[0].Words[1].Text != "好" || data[0].Words[1].End.MS != 2000 {
		t.Fatalf("unexpected second word: %#v", data[0].Words[1])
	}
}

func TestParseKRCWithLanguage(t *testing.T) {
	languageJSON := `{"content":[{"type":0,"lyricContent":[["ni","hao"]]},{"type":1,"lyricContent":[["hello"]]}]}`
	language := base64.StdEncoding.EncodeToString([]byte(languageJSON))
	raw := "[language:" + language + "]\n[1000,1000]<0,500,0>你<500,500,0>好"

	tags, data := ParseKRC(raw)
	if tags["language"] == "" {
		t.Fatal("missing language tag")
	}
	if data["roma"][0].Words[1].Text != "hao" {
		t.Fatalf("unexpected roma: %#v", data["roma"])
	}
	if data["ts"][0].Words[0].Text != "hello" {
		t.Fatalf("unexpected translation: %#v", data["ts"])
	}
}

func TestDecryptKRC(t *testing.T) {
	plain := "[1000,1000]<0,1000,0>Hi"
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write([]byte(plain)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	encrypted := append([]byte("krc1"), compressed.Bytes()...)
	for i := 4; i < len(encrypted); i++ {
		encrypted[i] ^= krcKey[(i-4)%len(krcKey)]
	}

	got, err := DecryptKRC(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q, want %q", got, plain)
	}
}
