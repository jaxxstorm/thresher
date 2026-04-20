## Context

`thresher analyze web --web-access tailnet` currently uses `tsnet.Server` to publish the web UI on the tailnet. That creates a separate tailnet node owned by the Thresher process, even though the surrounding workflow already depends on the host's existing `tailscaled` instance for live capture and Tailscale identity. The result is a mismatch between the intended operational model and the actual one: capture runs on the host's existing Tailscale device, but remote viewing appears through a second device identity.

This change needs to preserve the current user-facing analyze workflow, including local-only web mode, live analysis updates, model control, pause or resume, and the single capability gate `lbrlabs.com/cap/thresher`. It must not change the decoded packet substrate or packet-wrapper semantics; fields such as `path_id`, `snat`, `dnat`, nested `inner` traffic, and `disco_meta` remain untouched because the change is strictly about how the existing web UI is exposed remotely.

The main technical constraints are:
- the host may already have unrelated Tailscale Serve routes configured
- tailnet access must use the existing `tailscaled` identity rather than a new node
- the current web UI assumes root-relative routes, while Serve publication is safer under a dedicated mount path
- remote authorization can no longer assume direct ownership of the tailnet socket by the app

## Goals / Non-Goals

**Goals:**
- Replace the `tsnet`-backed remote web runtime with a Serve-backed runtime that uses the existing local `tailscaled` instance.
- Keep the Thresher web server bound to localhost and publish it remotely through Tailscale Serve.
- Preserve the single remote permission surface using `lbrlabs.com/cap/thresher`.
- Merge with existing host Serve config safely and remove only Thresher-owned routes on shutdown.
- Report a deterministic remote URL that reflects the host's existing tailnet identity.

**Non-Goals:**
- Changing packet parsing, capture semantics, JSONL output, or any wrapper-derived fields such as `path_id`, `inner`, `snat`, `dnat`, or `disco_meta`.
- Changing local-only web mode, the console workflow, batching, model discovery, or Aperture request semantics.
- Supporting multiple simultaneous tailnet-published analyze sessions on the same host in the first version.
- Expanding the capability model beyond the single `lbrlabs.com/cap/thresher` permission surface.

## Decisions

### Replace `tsnet.Server` with LocalAPI Serve config on the existing device
Tailnet web mode will stop creating a new `tsnet.Server`. Instead, Thresher will continue to run its own HTTP server on `127.0.0.1:<ephemeral-port>` and will use `tailscale.com/client/local` to read and write the host's Serve config through `GetServeConfig` and `SetServeConfig`.

This aligns tailnet web access with the existing capture path, which already depends on the local Tailscale daemon. It also removes the extra node identity from the tailnet and makes the published URL belong to the host's existing MagicDNS identity.

Alternative considered: keep `tsnet` and only adjust docs. Rejected because it would preserve the incorrect operational model and continue creating a second device identity.

### Publish under a dedicated Serve mount path
Remote publication will use a dedicated mount path under the host's HTTPS Serve surface, rather than taking over `/` or publishing from a separate Thresher-owned node. The design assumes a deterministic mount such as `/thresher/`, and the printed URL will point at that path on the host's existing tailnet name.

This reduces the chance of colliding with unrelated Serve usage on the same host and gives Thresher one stable permission surface to manage. It does require the web UI to understand a base path instead of assuming root-relative routes.

Alternative considered: publish on `/` or on a custom port. Root was rejected because it is more likely to conflict with unrelated Serve usage. A custom port was rejected because it is less discoverable and still requires shared Serve-config management.

### Use Serve-forwarded capability data for remote authorization
In the `tsnet` model, the app can inspect `r.RemoteAddr` directly and ask `WhoIs` about the tailnet peer. In the Serve model, `tailscaled` terminates the tailnet connection and proxies the request to localhost, so the backend should authorize based on Serve-forwarded identity and capability context instead.

The remote handler will therefore rely on the capability data forwarded by Serve for accepted application capabilities, specifically `lbrlabs.com/cap/thresher`. The localhost listener remains the only network listener Thresher owns directly, so local browser access can continue to work without remote capability checks while tailnet-served access is gated by forwarded Serve metadata.

Alternative considered: continue using `WhoIs(r.RemoteAddr)` directly. Rejected because proxied localhost requests do not preserve the original tailnet socket address in a reliable way for direct authorization.

### Merge and clean up only Thresher-owned Serve config
Serve config is shared host state. Thresher will fetch the existing config, merge in only the dedicated Thresher mount, and write it back with the returned ETag. On shutdown, it will remove only the route it owns and leave unrelated Serve config intact.

To make cleanup deterministic, the route shape and proxy target contract will be fixed and identifiable. If the expected mount is already occupied by unrelated config, Thresher should fail fast with a clear error rather than overwrite it.

Alternative considered: replace the entire Serve config when tailnet mode starts. Rejected because it would break unrelated host Serve usage and make rollback unsafe.

### Add explicit base-path awareness to the web UI
The current web UI assumes `/`, `/snapshot`, `/events`, and `/control/...` are rooted at the server origin. Under a Serve mount, the route tree needs to live under a prefix such as `/thresher/`. The presenter and browser code will therefore take a route prefix and emit links, EventSource URLs, and control POSTs relative to that prefix.

Alternative considered: keep root-relative handlers and use a root Serve mount. Rejected because it undermines the collision-avoidance decision above.

## Risks / Trade-offs

- [Shared Serve config can be changed concurrently by other tools] -> Use `GetServeConfig` / `SetServeConfig` with ETag handling and fail clearly on conflicts instead of overwriting blindly.
- [A stale Thresher route may remain after an unclean exit] -> Reconcile startup against the expected route shape and clean up only routes that match Thresher ownership semantics.
- [Serve-backed remote auth differs from direct socket auth] -> Base remote authorization on Serve-forwarded application capability data instead of direct `RemoteAddr` ownership assumptions.
- [A fixed mount path limits concurrent remote sessions on one host] -> Treat single active remote session per host as the supported model for this change and fail clearly on collisions.
- [Base-path support touches both backend handlers and browser JavaScript] -> Keep route-prefix plumbing centralized in the web presenter and cover it with path-aware web tests.

## Migration Plan

1. Add a new Serve-backed tailnet web runtime alongside the existing local runtime.
2. Refactor remote web route handling so all web endpoints can live under a configurable prefix.
3. Replace the `tsnet`-specific startup and authorization flow with LocalAPI Serve config management and Serve-forwarded capability handling.
4. Update CLI output, docs, and tests to describe the host-identity URL and the Serve-backed lifecycle.
5. Remove the `tsnet` dependency path once Serve-backed tailnet mode is verified.

Rollback is straightforward: restore the previous `tsnet` runtime and tailnet-auth path if Serve-backed publication proves too disruptive.

## Open Questions

- None for the initial implementation. This design assumes a dedicated `/thresher/` Serve mount on the host's existing HTTPS tailnet endpoint and a single active remote analyze session per host.
