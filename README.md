# hag — howls-agy, Antigravity CLI (agy) account switcher

Chạy nhiều account Google cho Antigravity CLI (`agy`) song song. Clone của `hcx` (howls-codex).

agy lưu token vào login keychain với key cố định (`gemini`/`antigravity`) nên không tách được theo thư mục.
Nhưng keychain search list đi theo `$HOME`: dưới HOME giả không có keychain, agy tự lưu token vào
`$HOME/.gemini/jetski-standalone-oauth-token`. Vì vậy mỗi account = 1 HOME giả.

- `main` = HOME thật (token trong keychain), `<name>` = `~/.agy-<name>` (dùng làm `HOME` khi chạy agy)
- HOME giả symlink mọi entry top-level của HOME thật (`.zshrc .gitconfig .ssh .config ...`), **trừ** `.gemini` và `Library`
- `.gemini` của account phụ symlink từ `~/.gemini`: `config` (hooks, mcp, plugins, skills, projects) `GEMINI.md settings.json`
  và `antigravity-cli/{settings.json history.jsonl conversations conversation_summaries.db jetbox_summaries_proto.pb brain annotations knowledge implicit plugin_data mcp scratch}`
- Riêng từng account: token, `jetski_state.pbtxt`, `installation_id`, `cache` (project mặc định), `log`, `crashes`, ...

Lưu ý:

- Không link `Library` (link sẽ kéo keychain vào lại, làm token dùng chung). Nên trong session account phụ, tool đọc keychain
  (git credential-osxkeychain, `gh` lưu token trong keychain) không chạy; dùng ssh key hoặc token file.
- Thêm file/thư mục mới vào HOME thật thì chạy lại `hag add <name>` để đồng bộ.
- Chưa có `hag quota`: endpoint quota của agy (`v1internal:retrieveUserQuotaSummary`) trả 429 khi thử, chưa xác minh được schema.

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
eval "$(hag env work)"    # set HOME + AGY_ACCOUNT cho shell hiện tại (đổi HOME cả shell)
```

Lần login đầu của account phụ, kiểm tra token đã tách: có file `~/.agy-work/.gemini/jetski-standalone-oauth-token`
và item keychain `gemini`/`antigravity` không đổi ngày sửa (`security find-generic-password -s gemini -a antigravity`).

Alias gợi ý trong `~/.zshrc`:

```sh
alias agy-w='hag work'
alias agy-w-yolo='hag work --dangerously-skip-permissions'
```

`$AGY_ACCOUNT` được set khi chạy, statusline/hook có thể đọc để hiện account.

Test: `go test ./...`
