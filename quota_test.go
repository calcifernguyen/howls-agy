package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatQuota(t *testing.T) {
	var q quotaSummary
	err := json.Unmarshal([]byte(`{"groups":[{"displayName":"Gemini Models","buckets":[
		{"bucketId":"gemini-5h","window":"5h","displayName":"Five Hour Limit Remaining","resetTime":"2026-09-24T14:49:20Z","remainingFraction":0.25},
		{"bucketId":"gemini-weekly","displayName":"Weekly Limit Remaining","resetTime":"2026-10-01T09:49:20Z"}]}]}`), &q)
	if err != nil {
		t.Fatal(err)
	}
	lines := formatQuota(q, false)
	if len(lines) != 3 || lines[0] != "  Gemini Models" ||
		!strings.Contains(lines[1], "5h      █████░░░░░░░░░░░░░░░   25%") || !strings.Contains(lines[2], "░░░░    0%") {
		t.Fatalf("got %q", lines) // remainingFraction vắng (proto3 bỏ số 0) => 0%
	}
}

func TestGeminiGroup(t *testing.T) {
	var m availableModels
	json.Unmarshal([]byte(`{"models":{
		"a":{"displayName":"Gemini 3 Flash","modelProvider":"MODEL_PROVIDER_GOOGLE","quotaInfo":{"remainingFraction":0.85,"resetTime":"2026-11-20T05:41:47Z"}},
		"b":{"displayName":"Gemini 2.5 Pro","modelProvider":"MODEL_PROVIDER_GOOGLE","quotaInfo":{"remainingFraction":0.85,"resetTime":"2026-11-20T05:41:47Z"}},
		"c":{"modelProvider":"MODEL_PROVIDER_GOOGLE","quotaInfo":{"remainingFraction":1}},
		"d":{"displayName":"Claude Opus","modelProvider":"MODEL_PROVIDER_ANTHROPIC","quotaInfo":{"resetTime":"2026-11-25T03:56:36Z"}}}}`), &m)
	g, ok := geminiGroup(m)
	if !ok || g.DisplayName != "Gemini models" || len(g.Buckets) != 1 || g.Buckets[0].RemainingFraction != 0.85 {
		t.Fatalf("got %+v", g)
	}
	if _, ok := geminiGroup(availableModels{}); ok {
		t.Fatal("không có model Google phải trả false")
	}
}

func TestReadTokenFile(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	p := filepath.Join(h, ".agy-work", tokenFile)
	os.MkdirAll(filepath.Dir(p), 0o700)
	os.WriteFile(p, []byte(`{"token":{"access_token":"a","refresh_token":"r","expiry":"2099-01-01T00:00:00.5+07:00"},"auth_method":"consumer"}`), 0o600)
	tok, err := readToken("work")
	if err != nil || tok.RefreshToken != "r" || tok.Expiry.Year() != 2099 {
		t.Fatalf("tok=%+v err=%v", tok, err)
	}
	if a, err := accessToken(tok); err != nil || a != "a" { // còn hạn => không refresh
		t.Fatalf("access=%q err=%v", a, err)
	}
	if _, err := readToken("none"); err == nil {
		t.Fatal("account chưa login phải lỗi")
	}
}

func TestBarColor(t *testing.T) {
	for frac, code := range map[float64]string{0.9: "32", 0.3: "33", 0.1: "31"} {
		if b := bar(frac, true); !strings.HasPrefix(b, "\x1b["+code+"m") || !strings.HasSuffix(b, "\x1b[0m") {
			t.Errorf("%v: %q", frac, b)
		}
	}
}
