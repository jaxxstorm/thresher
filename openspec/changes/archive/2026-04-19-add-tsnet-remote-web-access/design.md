## Context

`thresher analyze --mode web` already has an in-process HTTP presenter, but it is intentionally bound to localhost and assumes the operator is on the same machine as the capture session. The new requirement is to keep the existing live-capture, batching, model selection, pause, and streamed-analysis behavior while making that same web session reachable from other devices on the tailnet.

This crosses command wiring, HTTP serving, Tailscale integration, and request authorization. It also needs to avoid broadening exposure by accident: localhost web mode must remain the default, and the remote path must be gated by a single capability key, `lbrlabs.com/cap/thresher`, because the browser surface is still one logical application.

## Goals / Non-Goals

**Goals:**
- Add an explicit way to start web mode in either local-only or tailnet-served form.
- Reuse the existing analyze session engine and web presenter state so local and remote web mode show the same counters, model controls, pause controls, and live analysis output.
- Serve the remote UI over `tsnet.Server` and allow only peers with `lbrlabs.com/cap/thresher`.
- Keep the remote capability contract simple by using one permission surface for the entire web UI.

**Non-Goals:**
- Replacing the existing console mode or making remote access the default for web mode.
- Changing packet wrapper semantics, LocalAPI capture behavior, or Aperture request payloads.
- Building a general remote API, multi-page application, or per-endpoint capability matrix.
- Adding non-Tailscale authentication, public internet exposure, or external reverse-proxy requirements.

## Decisions

### Add a second web exposure control instead of overloading `--mode`
`--mode` should continue to choose the presentation surface (`console` or `web`). A separate web access setting should choose whether web mode is `local` or `tailnet`, with `local` as the default.

This keeps the meaning of `--mode web` stable for existing users and makes remote exposure an explicit opt-in. It also leaves room for config defaults without conflating browser presentation with network reachability.

Alternative considered: add a third mode such as `--mode web-remote`. This was rejected because it mixes transport concerns into the presenter selector and makes future web-mode variations harder to compose.

### Reuse the existing web presenter handlers behind interchangeable listeners
The current web presenter already owns the page, snapshot, events, and control handlers. Remote access should preserve those semantics and swap only the listener/runtime layer so the same handler tree can be served either by a localhost `net.Listener` or a `tsnet.Server` listener.

This avoids duplicating the analysis web UI and keeps pause, model selection, quit, and live updates identical across local and remote browser sessions.

Alternative considered: create a separate remote presenter. This was rejected because it would duplicate the same HTTP routes and increase the chance that local and remote web mode diverge.

### Gate every tailnet request with one capability key and one path surface
Remote requests should be authorized by checking the connecting tailnet identity against `lbrlabs.com/cap/thresher`. The remote web surface should stay under one root path so the capability contract is still one logical permission endpoint even though the implementation serves HTML, snapshot, events, and control routes beneath it.

This matches the user requirement that there is only one web page and therefore only one permission surface to describe in the capability policy. The authorization middleware can run on every request, but it should always evaluate the same capability and path scope rather than a per-route capability map.

Alternative considered: separate capability names or path grants for `/events`, `/snapshot`, and control endpoints. This was rejected because it adds policy complexity without adding a distinct user-facing surface.

### Keep capture and analysis execution on the remote host
Remote web access should not change where capture happens. The analyze session should still run on the host where `thresher analyze` was invoked, using that machine's LocalAPI capture path and Aperture endpoint connectivity, while remote browsers only observe and control the already-running session.

This preserves the current single-process execution model and avoids introducing distributed capture forwarding or browser-side packet handling.

Alternative considered: proxy packet data or analysis control through a separate coordinator. This was rejected because the immediate use case is remote visibility into a session already running near the target machine.

### Advertise an explicit tailnet URL when remote mode starts
When tailnet exposure is enabled, the command should print the resolved tailnet URL after the tsnet listener starts, just as localhost web mode currently prints the local URL. The advertised URL should be stable enough for a user to open from another tailnet device during the session.

Alternative considered: rely on logs or out-of-band hostname discovery. This was rejected because the operator needs an immediate browser target when starting a remote capture session.

## Risks / Trade-offs

- [Remote exposure broadens the attack surface compared with localhost-only mode] -> Keep local mode as the default and require the `lbrlabs.com/cap/thresher` capability for every tailnet request.
- [Local and remote web mode can drift if handlers fork] -> Reuse one handler tree and keep transport-specific behavior at the listener/auth middleware layer.
- [Capability checks may be subtle to test] -> Add focused HTTP tests that exercise allowed and denied remote requests plus shared control actions.
- [Tailnet URL construction may vary by environment] -> Centralize URL resolution around the chosen tsnet hostname/listener and surface the resolved URL from the runtime rather than rebuilding it in multiple places.
- [Long-running sessions may leave tsnet or HTTP listeners behind on quit or failure] -> Tie both the HTTP server and tsnet server shutdown to the same presenter/session lifecycle.

## Migration Plan

No data migration is required. The implementation should add the new web-access control and tsnet runtime behind the existing `analyze` command while keeping localhost web mode unchanged by default. Rollback is straightforward: disable the tailnet-serving branch and retain the current localhost presenter.

## Open Questions

- Should the tsnet hostname be fixed by default, derived from the machine name, or exposed as a user-facing flag from the start?
- Should remote mode continue to use plain HTTP on the tailnet, or should the first implementation terminate HTTPS directly through tsnet if the runtime makes that straightforward?
