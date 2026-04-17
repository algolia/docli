# AGENTS

## Toolchain
- Preferred local task runner is `mise`; versions are pinned in `mise.toml` (`go 1.26`, `golangci-lint 2.11.4`, `goreleaser 2.14.3`).
- Build with `mise run build` or `go build -buildvcs=false`. The `-buildvcs=false` flag is intentional for worktrees.
- Format with `mise run format` (`golangci-lint fmt`), not plain `gofmt`.

## Verification
- Full repo checks: `mise run lint` and `mise run test`. CI runs `golangci-lint run` and `go test ./...`.
- Focused test example: `go test ./pkg/cmd/generate/clients -run TestGetAPIDataRendersQuotedPlainTextDescription`.
- If you change Cobra commands, help text, or docs generator code under `pkg/cmd/**` or `internal/docs/**`, run `mise run readme` to regenerate `README.md`; CI has a workflow that opens a PR when this is missed.

## Structure
- Entry point is `main.go` -> `pkg/cmd/root`. CLI subcommands are wired in `pkg/cmd/generate/index.go`.
- Keep command behavior in `pkg/cmd/generate/<command>/`; shared CLI output flags and file writes live in `pkg/output`.
- Validation helpers are centralized in `pkg/validate`; reuse them instead of open-coding path checks.
- `internal/docs/main.go` rebuilds `README.md` from `internal/docs/usage.md` plus the live Cobra command tree.

## Repo-Specific Behavior
- Global flags are defined on the root command: `--dry-run`, `--verbose`, `--quiet`. `--verbose` and `--quiet` are mutually exclusive in `pkg/output`.
- Generated OpenAPI/client pages are built from embedded `*.mdx.tmpl` templates via `go:embed`; command tests assert rendered frontmatter/content, so template changes usually need test updates.
- `pkg/cmd/generate/page_templates_test.go` enforces `public: true` in built-in page templates (`clients`, `openapi`, `sla`).
- `generate openapi` and `generate clients` do not delete stale output files when operations are removed or renamed.
- `generate cdn` is the only generator here that reaches external services (npm registry and jsDelivr); keep tests for that code isolated from live network behavior.

## Release Notes
- `mise run bump` runs `format`, `lint`, and `test`, then executes `internal/bump`.
- `internal/bump` requires a clean git tree and computes semver from commit messages: breaking change header/body -> major, `feat:` -> minor, everything else -> patch.
- Goreleaser injects the CLI version into `pkg/cmd/root.version`; the default in source is `dev`.
