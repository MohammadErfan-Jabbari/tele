# tele (oikos fork)

This repository is Erfan's fork of [sorokin-vladimir/tele](https://github.com/sorokin-vladimir/tele) (`upstream`), maintained as the Telegram MTProto connector for `oikos` (`MohammadErfan-Jabbari/oikos#19`). `AGENTS.md` is a symlink to this file so Claude Code and Antigravity read the same instructions.

## Purpose and upstream discipline

`tele` is a single static Go binary (`CGO_ENABLED=0`, `gotd/td`, pure-Go SQLite) with a single-owner `flock` daemon over a `0600` Unix domain socket, contiguous-run SQLite caching, dual-lane flood throttling, and host-side `cli.account_commands: read` enforcement by default.

Keep fork additions small and modular (prefer dedicated files and packages over wide edits to upstream files) so periodic `git fetch upstream && git rebase upstream/main` or merges in the `oikos` upkeep loop stay clean. Never commit `config.yml`, `session.json`, `state.db`, `api_id`, `api_hash`, or real message contents.

## Build, test, and lint on `dev`

Go `1.27.4` is installed on `dev` through Linuxbrew. Put it on `PATH` before running Go commands:

```bash
export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"
CGO_ENABLED=0 go build -o /tmp/tele ./cmd/tele
go vet ./...
go test -count=1 ./cmd/tele/... ./internal/appkey/... ./internal/ipc/...
go test -count=1 ./...
gofmt -l .
```

## Architecture invariants (enforced by `.golangci.yml` and tests)

- **Layer boundary (`depguard`).** `internal/ui/` must never import `internal/store` or `internal/tg`. The UI and CLI reach Telegram only through `core.Surface` over IPC.
- **Binary size (`forbidigo`, `cmd/tele/size_test.go`).** Never use `reflect.{Type,Value}.Method*` or `text/template` in `cmd/`, because reflection keeps every generated `gotd/td` method and doubles the binary size.
- **Terminal ownership (`forbidigo`).** Never call `fmt.Print*` outside `cmd/`; write to an injected `io.Writer` or the zap logger.
- **IPC RPC partition (`internal/ipc/protocol/surface.go`).** Every `core.Surface` method belongs to either `protocol.Reads` or `protocol.Writes` (verified by tests), and `AccessHash` plus `FileReference` are stripped before crossing the IPC boundary.
- **CLI exit codes (`docs/cli.md`).** `0` success, `1` general error, `2` usage or bad chat identifier, `3` no host running, `4` not logged in, `5` chat/topic/message not found, `6` permanent send failure, `7` refused by `cli.account_commands`, `75` transient wait or restart.

## Fork-specific safety and `oikos` requirements

- **Credentials.** Read `TELEGRAM_API_ID` and `TELEGRAM_API_HASH` from the environment (injected by `treasury`) without requiring plaintext secrets in `config.yml`.
- **Read-only boundary.** Default to read-only (`cli.account_commands: read`), enforce `TELE_READ_ONLY=1` across Linux `/proc/<pid>/environ` ancestors, and block `$restart` and `$uninstall` from `KindCommand` clients.
- **Untrusted input.** Sanitize control and bidi characters on CLI output (preserving `\n`, `\t`, emoji ZWJ, and Persian/Arabic ZWNJ) and redact Telegram login codes (`777000`/`489000`) and gift links (`t.me/giftcode`, `tg://giftcode`) before agent consumption.

## Development workflow

Development runs on the factory (`factory@factory`), enabled in `.claude/settings.json`. Follow `~/workspace/agent-config/docs/factory.md` for model mapping and tool equivalents (do not overwrite with `setup-pstack`).

- Start every bug fix or feature with failing-first Go tests.
- Stage changes by explicit path; never run `git add -A` or `git commit -a`.
- Open a ready-for-review pull request against `MohammadErfan-Jabbari/tele:main` and verify it before merging; never push directly to `main`.
- Keep `oikos` docs and replies following `unslop` and `technical-writing`.
