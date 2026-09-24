// hag: chạy nhiều account Antigravity CLI (agy) song song bằng HOME giả.
// main = HOME thật, <name> = ~/.agy-<name>. agy lưu token vào login keychain (key cố định gemini/antigravity),
// nhưng keychain search list đi theo $HOME: dưới HOME giả không có keychain nên agy tự lưu token vào
// $HOME/.gemini/antigravity-cli/antigravity-oauth-token => token tách theo account.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
)

// Item trong ~/.gemini symlink sang account phụ (đường dẫn tương đối ~/.gemini).
var shared = []string{
	"config", "GEMINI.md", "settings.json",
	"antigravity-cli/settings.json", "antigravity-cli/history.jsonl", "antigravity-cli/conversations",
	"antigravity-cli/conversation_summaries.db", "antigravity-cli/jetbox_summaries_proto.pb",
	"antigravity-cli/brain", "antigravity-cli/annotations", "antigravity-cli/knowledge", "antigravity-cli/implicit",
	"antigravity-cli/plugin_data", "antigravity-cli/mcp", "antigravity-cli/scratch",
}

// Item nguồn chưa có thì tạo thư mục; file nguồn chưa có thì bỏ qua (file rỗng có thể làm agy lỗi parse).
var sharedDirs = map[string]bool{
	"config": true, "antigravity-cli/conversations": true, "antigravity-cli/brain": true,
	"antigravity-cli/annotations": true, "antigravity-cli/knowledge": true, "antigravity-cli/implicit": true,
	"antigravity-cli/plugin_data": true, "antigravity-cli/mcp": true, "antigravity-cli/scratch": true,
}

// Entry top-level của HOME thật KHÔNG symlink sang HOME giả. Library kéo theo Keychains/Preferences => lộ keychain => chung token.
var homeSkip = map[string]bool{".gemini": true, "Library": true}

const tokenFile = ".gemini/antigravity-cli/antigravity-oauth-token"
const aliasFlags = "--dangerously-skip-permissions"

const usage = `hag — Antigravity CLI (agy) account switcher

  hag add <name>            tạo/đồng bộ ~/.agy-<name> (HOME giả), symlink HOME thật + shared trong ~/.gemini
  hag list                  liệt kê account (* = đang active theo $HOME)
  hag env <name>            in lệnh export, dùng: eval "$(hag env <name>)"
  hag alias [name]          in alias zsh gợi ý, dùng: eval "$(hag alias)"
  hag quota [name]          quota còn lại (mặc định mọi account)
  hag <name> [args...]      chạy agy với account <name>`

// home: HOME thật. Gọi lồng trong session account phụ (HOME=~/.agy-x) vẫn ra HOME thật.
func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		die(err)
	}
	if strings.HasPrefix(filepath.Base(h), ".agy-") {
		return filepath.Dir(h)
	}
	return h
}

func mainDir() string { return filepath.Join(home(), ".gemini") }

func accountHome(name string) string {
	if name == "main" {
		return home()
	}
	return filepath.Join(home(), ".agy-"+name)
}

func die(v any) {
	fmt.Fprintln(os.Stderr, "hag:", v)
	os.Exit(1)
}

func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\. `) && !strings.HasPrefix(name, "-")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Println(usage)
		return
	}
	var err error
	switch args[0] {
	case "add":
		if len(args) != 2 {
			die("usage: hag add <name>")
		}
		err = cmdAdd(args[1])
	case "list", "ls":
		err = cmdList()
	case "env":
		if len(args) != 2 {
			die("usage: hag env <name>")
		}
		err = cmdEnv(args[1])
	case "alias":
		if len(args) > 2 {
			die("usage: hag alias [name]")
		}
		err = cmdAlias(args[1:])
	case "quota":
		if len(args) > 2 {
			die("usage: hag quota [name]")
		}
		err = cmdQuota(args[1:])
	default:
		err = run(args[0], args[1:])
	}
	if err != nil {
		die(err)
	}
}

func cmdAdd(name string) error {
	if !validName(name) || name == "main" {
		return fmt.Errorf("tên không hợp lệ: %q", name)
	}
	h := accountHome(name)
	if err := os.MkdirAll(filepath.Join(h, ".gemini"), 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(home())
	if err != nil {
		return err
	}
	for _, e := range entries {
		if homeSkip[e.Name()] || strings.HasPrefix(e.Name(), ".agy-") {
			continue
		}
		if err := link(filepath.Join(home(), e.Name()), filepath.Join(h, e.Name())); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	for _, item := range shared {
		if err := linkShared(item, filepath.Join(h, ".gemini")); err != nil {
			return fmt.Errorf("%s: %w", item, err)
		}
	}
	fmt.Printf("OK: %s. Login: hag %s\n", h, name)
	return nil
}

// linkShared: dir/item -> ~/.gemini/item. Thư mục nguồn chưa có thì tạo, file nguồn chưa có thì bỏ qua.
func linkShared(item, dir string) error {
	src := filepath.Join(mainDir(), item)
	if _, err := os.Lstat(src); os.IsNotExist(err) {
		if !sharedDirs[item] {
			return nil
		}
		if err := os.MkdirAll(src, 0o755); err != nil {
			return err
		}
	}
	dst := filepath.Join(dir, item)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return link(src, dst)
}

// link: dst -> src. Idempotent; item thật ở dst bị rename sang .hag-bak-<unix>, không xoá.
func link(src, dst string) error {
	if cur, err := os.Readlink(dst); err == nil && cur == src {
		return nil
	}
	if _, err := os.Lstat(dst); err == nil {
		bak := fmt.Sprintf("%s.hag-bak-%d", dst, time.Now().Unix())
		if err := os.Rename(dst, bak); err != nil {
			return err
		}
		fmt.Printf("backup: %s -> %s\n", dst, bak)
	}
	return os.Symlink(src, dst)
}

func accounts() []string {
	names := []string{"main"}
	matches, _ := filepath.Glob(filepath.Join(home(), ".agy-*"))
	for _, m := range matches {
		// Account = đã login (token file) hoặc tạo bởi hag (.gemini/config là symlink).
		_, login := os.Stat(filepath.Join(m, tokenFile))
		fi, lerr := os.Lstat(filepath.Join(m, ".gemini", "config"))
		if login == nil || (lerr == nil && fi.Mode()&os.ModeSymlink != 0) {
			names = append(names, strings.TrimPrefix(filepath.Base(m), ".agy-"))
		}
	}
	return names
}

var emailRe = regexp.MustCompile(`applyAuthResult: email=([^,\s]+)`)

// email: token không có id_token nên lấy từ log agy (dòng applyAuthResult mới nhất).
func email(h string) string {
	logs, _ := filepath.Glob(filepath.Join(h, ".gemini", "antigravity-cli", "log", "cli-*.log"))
	slices.Reverse(logs) // tên cli-YYYYMMDD_HHMMSS.log => mới nhất trước
	for _, l := range logs {
		b, err := os.ReadFile(l)
		if err != nil {
			continue
		}
		if m := emailRe.FindAllSubmatch(b, -1); len(m) > 0 {
			return string(m[len(m)-1][1])
		}
	}
	return "(không rõ)"
}

func cmdList() error {
	active := filepath.Clean(os.Getenv("HOME"))
	for _, name := range accounts() {
		h := accountHome(name)
		mark := " "
		if h == active {
			mark = "*"
		}
		fmt.Printf("%s %-10s %-30s %s\n", mark, name, email(h), h)
	}
	return nil
}

func resolve(name string) (string, error) {
	if !validName(name) {
		return "", fmt.Errorf("tên không hợp lệ: %q\n%s", name, usage)
	}
	h := accountHome(name)
	if _, err := os.Stat(h); err != nil {
		return "", fmt.Errorf("account %q chưa có, chạy: hag add %s", name, name)
	}
	return h, nil
}

func cmdEnv(name string) error {
	h, err := resolve(name)
	if err != nil {
		return err
	}
	fmt.Printf("export HOME=%q AGY_ACCOUNT=%q\n", h, name)
	return nil
}

func aliasLine(name string) string {
	if name == "main" {
		return fmt.Sprintf("alias agy='hag main %s'", aliasFlags)
	}
	return fmt.Sprintf("alias agy-%s='hag %s %s'", name, name, aliasFlags)
}

func cmdAlias(names []string) error {
	if len(names) == 0 {
		for _, name := range accounts() {
			fmt.Println(aliasLine(name))
		}
		return nil
	}
	if _, err := resolve(names[0]); err != nil {
		return err
	}
	fmt.Println(aliasLine(names[0]))
	return nil
}

func run(name string, args []string) error {
	h, err := resolve(name)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath("agy")
	if err != nil {
		return err
	}
	env := []string{"HOME=" + h, "AGY_ACCOUNT=" + name}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "AGY_ACCOUNT=") {
			env = append(env, kv)
		}
	}
	return syscall.Exec(bin, append([]string{"agy"}, args...), env)
}
