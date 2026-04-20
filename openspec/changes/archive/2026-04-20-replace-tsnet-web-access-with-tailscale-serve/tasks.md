## 1. Serve Runtime

- [x] 1.1 Replace the tailnet runtime in `internal/analyze/web_runtime.go` so `analyze web --web-access tailnet` uses `tailscale.com/client/local` with `GetServeConfig` and `SetServeConfig`, publishing a localhost web listener through the existing host `tailscaled` identity without changing decoded packet fields such as `path_id`, `inner`, or `disco_meta`
- [x] 1.2 Add runtime-focused tests in `internal/analyze/web_test.go` for Serve-config merge, ETag-safe updates, cleanup of only the Thresher-owned route, and preserved URL reporting on the host identity

## 2. Route Prefix And Remote Auth

- [x] 2.1 Update `internal/analyze/web.go` and related runtime wiring so the remote web UI runs under one dedicated Serve-backed path prefix and authorizes remote requests using forwarded Tailscale application capability data for `lbrlabs.com/cap/thresher` instead of direct `tsnet` socket ownership assumptions
- [x] 2.2 Add web-handler tests in `internal/analyze/web_test.go` and related files to cover prefixed page, snapshot, events, model, pause, and quit routes, plus allow or deny behavior when the forwarded capability data is present or absent

## 3. CLI And Documentation

- [x] 3.1 Update `cmd/analyze.go`, related startup messaging, and `docs/analyze-command.md` so tailnet web mode is described as Serve-backed access on the existing Tailscale device rather than a separate Thresher-managed node, with clear failure behavior when the dedicated remote route cannot be claimed
- [x] 3.2 Run `go test ./cmd ./internal/analyze` and manual smoke tests for `go run . analyze web --web-access local` and `go run . analyze web --web-access tailnet`, confirming the remote Serve path works without changing prompt content built from wrapper-derived fields such as `path_id`, nested `inner`, `snat`, `dnat`, and `disco_meta`
