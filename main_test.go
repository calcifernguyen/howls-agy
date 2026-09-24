package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddIdempotentWithBackup(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	g := filepath.Join(h, ".gemini")
	os.MkdirAll(filepath.Join(g, "config"), 0o755)
	os.MkdirAll(filepath.Join(g, "antigravity-cli"), 0o755)
	os.WriteFile(filepath.Join(g, "antigravity-cli", "settings.json"), []byte("{}"), 0o600)
	os.MkdirAll(filepath.Join(h, "Library", "Keychains"), 0o755)
	os.WriteFile(filepath.Join(h, ".gitconfig"), []byte("x"), 0o644)
	acc := filepath.Join(h, ".agy-work")
	os.MkdirAll(filepath.Join(acc, ".gemini", "antigravity-cli"), 0o755)
	old := filepath.Join(acc, ".gemini", "antigravity-cli", "settings.json")
	os.WriteFile(old, []byte("old"), 0o600) // file thật phải bị backup

	for i := 0; i < 2; i++ {
		if err := cmdAdd("work"); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range shared {
		src := filepath.Join(g, item)
		if _, err := os.Stat(src); err != nil {
			continue // file nguồn chưa có => bỏ qua
		}
		if got, err := os.Readlink(filepath.Join(acc, ".gemini", item)); err != nil || got != src {
			t.Errorf("%s: link=%q err=%v, want %q", item, got, err, src)
		}
	}
	if _, err := os.Lstat(filepath.Join(acc, ".gemini", "GEMINI.md")); err == nil {
		t.Error("file nguồn chưa có không được tạo link")
	}
	if got, _ := os.Readlink(filepath.Join(acc, ".gitconfig")); got != filepath.Join(h, ".gitconfig") {
		t.Errorf(".gitconfig link = %q", got)
	}
	for _, p := range []string{"Library", ".gemini", ".agy-work"} {
		if fi, err := os.Lstat(filepath.Join(acc, p)); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s không được symlink", p)
		}
	}
	if baks, _ := filepath.Glob(old + ".hag-bak-*"); len(baks) != 1 {
		t.Errorf("want 1 backup, got %v", baks)
	}
	os.MkdirAll(filepath.Join(h, ".agy-other"), 0o755) // không token, không symlink -> bỏ qua
	if got := accounts(); len(got) != 2 || got[1] != "work" {
		t.Errorf("accounts = %v", got)
	}
	if cmdAdd("main") == nil || cmdAdd("../x") == nil {
		t.Error("tên không hợp lệ phải lỗi")
	}
}

func TestHomeNested(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", filepath.Join(h, ".agy-w"))
	if got := home(); got != h {
		t.Errorf("home() = %q, want %q", got, h)
	}
	if got := accountHome("w"); got != filepath.Join(h, ".agy-w") {
		t.Errorf("accountHome(w) = %q", got)
	}
}

func TestValidName(t *testing.T) {
	for name, want := range map[string]bool{"work": true, "w2": true, "": false, "-x": false, "a/b": false, "a.b": false, "a b": false} {
		if validName(name) != want {
			t.Errorf("validName(%q) != %v", name, want)
		}
	}
}

func TestEmail(t *testing.T) {
	h := t.TempDir()
	if got := email(h); got != "(không rõ)" {
		t.Errorf("no log: %q", got)
	}
	dir := filepath.Join(h, ".gemini", "antigravity-cli", "log")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "cli-20260101_000000.log"), []byte("I0101 oauth.go:196] applyAuthResult: email=old@x.com, authMethod=consumer\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cli-20260201_000000.log"), []byte(
		"I0201 oauth.go:196] applyAuthResult: email=a@x.com, authMethod=consumer\n"+
			"I0201 oauth.go:196] applyAuthResult: email=b@x.com, authMethod=consumer\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cli-20260301_000000.log"), []byte("not logged in\n"), 0o644)
	if got := email(h); got != "b@x.com" {
		t.Errorf("email = %q", got)
	}
}
