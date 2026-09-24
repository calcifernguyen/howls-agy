# hag — howls-agy, Antigravity CLI (agy) account switcher

Chạy nhiều account Google cho Antigravity CLI (`agy`) song song. Clone của `hcx` (howls-codex).

agy lưu token vào login keychain với key cố định (`gemini`/`antigravity`) nên không tách được theo thư mục.
Nhưng keychain search list đi theo `$HOME`: dưới HOME giả không có keychain, agy tự lưu token vào
`$HOME/.gemini/antigravity-cli/antigravity-oauth-token`. Vì vậy mỗi account = 1 HOME giả.

- `main` = HOME thật (token trong keychain), `<name>` = `~/.agy-<name>` (dùng làm `HOME` khi chạy agy)
- HOME giả symlink mọi entry top-level của HOME thật (`.zshrc .gitconfig .ssh .config ...`), **trừ** `.gemini` và `Library`
- `.gemini` của account phụ symlink từ `~/.gemini`: `config` (hooks, mcp, plugins, skills, projects) `GEMINI.md settings.json`
  và `antigravity-cli/{settings.json history.jsonl conversations conversation_summaries.db jetbox_summaries_proto.pb brain annotations knowledge implicit plugin_data mcp scratch}`
- Riêng từng account: token, `jetski_state.pbtxt`, `installation_id`, `cache` (project mặc định), `log`, `crashes`, ...

Lưu ý:

- Không link `Library` (link sẽ kéo keychain vào lại, làm token dùng chung). Nên trong session account phụ, tool đọc keychain
  (git credential-osxkeychain, `gh` lưu token trong keychain) không chạy; dùng ssh key hoặc token file.
- Thêm file/thư mục mới vào HOME thật thì chạy lại `hag add <name>` để đồng bộ.
- `hag quota` gọi `v1internal:retrieveUserQuotaSummary` như agy. Server nhận diện client theo User-Agent: phải là `antigravity/<ver> <os>/<arch>`,
  UA khác bị trả `UNSUPPORTED_CLIENT` / `403 SUBSCRIPTION_REQUIRED`. Token hết hạn thì tự refresh (không ghi đè token của agy),
  cần env `HAG_OAUTH_CLIENT_ID` / `HAG_OAUTH_CLIENT_SECRET` = OAuth client installed-app của agy (tìm trong binary agy:
  `grep -aoE 'GOCSPX-[A-Za-z0-9_-]{28}|[0-9]+-[a-z0-9]{32}\.apps\.googleusercontent\.com' "$(command -v agy)"`).

## Cài

```sh
go build -o ~/.local/bin/hag .
```

## Dùng

```sh
hag add work              # tạo/đồng bộ ~/.agy-work; item thật cũ -> <item>.hag-bak-<unix>
hag work                  # chạy agy bằng account work (lần đầu: login trong agy)
hag main -c               # args sau tên account chuyển thẳng cho agy
hag list                  # * = account theo $HOME hiện tại; email lấy từ log agy
hag quota [work]          # quota còn lại (5h, weekly), thanh màu xanh/vàng/đỏ; NO_COLOR=1 hoặc pipe thì không màu
eval "$(hag env work)"    # set HOME + AGY_ACCOUNT cho shell hiện tại (đổi HOME cả shell)
eval "$(hag alias)"       # nạp alias cho shell hiện tại (hoặc: hag alias >> ~/.zshrc)
```

Lần login đầu của account phụ, kiểm tra token đã tách: có file `~/.agy-work/.gemini/antigravity-cli/antigravity-oauth-token`
và item keychain `gemini`/`antigravity` không đổi ngày sửa (`security find-generic-password -s gemini -a antigravity`).

Alias gợi ý trong `~/.zshrc`: thêm bằng `hag alias >> ~/.zshrc` hoặc nạp bằng `eval "$(hag alias)"`.

```sh
alias agy='hag main --dangerously-skip-permissions'
alias agy-work='hag work --dangerously-skip-permissions'
```

`$AGY_ACCOUNT` được set khi chạy, statusline/hook có thể đọc để hiện account.

Test: `go test ./...`
