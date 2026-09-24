package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const cloudcodeURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:"

// Server nhận diện client theo User-Agent: UA khác "antigravity/..." => UNSUPPORTED_CLIENT / 403 SUBSCRIPTION_REQUIRED.
// ponytail: version hard-code, đổi theo `agy --version` nếu server bắt đúng version.
var userAgent = "antigravity/1.2.10 " + runtime.GOOS + "/" + runtime.GOARCH

var httpClient = &http.Client{Timeout: 60 * time.Second}

type oauthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

type quotaSummary struct {
	Groups []struct {
		DisplayName string `json:"displayName"`
		Buckets     []struct {
			Window            string    `json:"window"`
			ResetTime         time.Time `json:"resetTime"`
			RemainingFraction float64   `json:"remainingFraction"`
		} `json:"buckets"`
	} `json:"groups"`
}

// readToken: token file của account; main không có file thì đọc keychain (go-keyring: "go-keyring-base64:<json>").
func readToken(name string) (oauthToken, error) {
	b, err := os.ReadFile(filepath.Join(accountHome(name), tokenFile))
	if os.IsNotExist(err) && name == "main" {
		var out []byte
		out, err = exec.Command("security", "find-generic-password", "-s", "gemini", "-a", "antigravity", "-w").Output()
		s := strings.TrimSpace(string(out))
		if raw, ok := strings.CutPrefix(s, "go-keyring-base64:"); ok && err == nil {
			b, err = base64.StdEncoding.DecodeString(raw)
		} else {
			b = []byte(s)
		}
	}
	if err != nil {
		return oauthToken{}, fmt.Errorf("chưa login (%v)", err)
	}
	var f struct {
		Token oauthToken `json:"token"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return oauthToken{}, fmt.Errorf("token lỗi format: %w", err)
	}
	return f.Token, nil
}

// accessToken: còn hạn thì dùng luôn, hết hạn thì refresh (không ghi đè token của agy).
func accessToken(t oauthToken) (string, error) {
	if time.Until(t.Expiry) > time.Minute {
		return t.AccessToken, nil
	}
	// OAuth client installed-app của agy (lấy từ binary agy), không commit vào repo.
	id, secret := os.Getenv("HAG_OAUTH_CLIENT_ID"), os.Getenv("HAG_OAUTH_CLIENT_SECRET")
	if id == "" || secret == "" {
		return "", fmt.Errorf("token hết hạn, cần set HAG_OAUTH_CLIENT_ID và HAG_OAUTH_CLIENT_SECRET để refresh")
	}
	resp, err := httpClient.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {id},
		"client_secret": {secret},
		"refresh_token": {t.RefreshToken},
	})
	if err != nil {
		return "", err
	}
	var r struct {
		AccessToken string `json:"access_token"`
	}
	if err := decode(resp, &r); err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}
	return r.AccessToken, nil
}

func decode(resp *http.Response, out any) error {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.Join(strings.Fields(string(b)), " "))
	}
	return json.Unmarshal(b, out)
}

func cloudcode(method, tok string, body, out any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", cloudcodeURL+method, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if err := decode(resp, out); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}

func fetchQuota(name string) (quotaSummary, error) {
	var q quotaSummary
	t, err := readToken(name)
	if err != nil {
		return q, err
	}
	tok, err := accessToken(t)
	if err != nil {
		return q, err
	}
	// cloudaicompanionProject: string hoặc {id}, có thể rỗng.
	var lca struct {
		Project json.RawMessage `json:"cloudaicompanionProject"`
	}
	meta := map[string]any{"metadata": map[string]string{"ideType": "ANTIGRAVITY", "pluginType": "GEMINI"}}
	if err := cloudcode("loadCodeAssist", tok, meta, &lca); err != nil {
		return q, err
	}
	var project string
	if json.Unmarshal(lca.Project, &project) != nil {
		var p struct{ ID string }
		json.Unmarshal(lca.Project, &p)
		project = p.ID
	}
	err = cloudcode("retrieveUserQuotaSummary", tok, map[string]string{"project": project}, &q)
	return q, err
}

const barWidth = 20

// bar: thanh quota còn lại. Màu theo mức: >=50% xanh, >=20% vàng, <20% đỏ; phần đã dùng xám mờ. Số % luôn in kèm nên không phụ thuộc màu.
func bar(frac float64, color bool) string {
	n := int(math.Round(min(max(frac, 0), 1) * barWidth))
	full, empty := strings.Repeat("█", n), strings.Repeat("░", barWidth-n)
	if !color {
		return full + empty
	}
	c := "32" // xanh
	switch {
	case frac < 0.2:
		c = "31" // đỏ
	case frac < 0.5:
		c = "33" // vàng
	}
	return "\x1b[" + c + "m" + full + "\x1b[90m" + empty + "\x1b[0m"
}

func formatQuota(q quotaSummary, color bool) []string {
	var lines []string
	for _, g := range q.Groups {
		lines = append(lines, "  "+g.DisplayName)
		for _, b := range g.Buckets {
			lines = append(lines, fmt.Sprintf("    %-7s %s %4.0f%%  reset %s",
				b.Window, bar(b.RemainingFraction, color), b.RemainingFraction*100, b.ResetTime.Local().Format("01-02 15:04")))
		}
	}
	return lines
}

// useColor: chỉ tô màu khi stdout là terminal và không set NO_COLOR.
func useColor() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == ""
}

func cmdQuota(names []string) error {
	if len(names) == 0 {
		names = accounts()
	} else if _, err := resolve(names[0]); err != nil {
		return err
	}
	color := useColor()
	for _, name := range names {
		head := name + " " + email(accountHome(name))
		if color {
			head = "\x1b[1m" + head + "\x1b[0m"
		}
		fmt.Println(head)
		q, err := fetchQuota(name)
		if err != nil {
			fmt.Printf("  lỗi: %v\n", err)
			continue
		}
		for _, l := range formatQuota(q, color) {
			fmt.Println(l)
		}
	}
	return nil
}
