## Why

Remote analyze web access currently creates a new tailnet device through `tsnet`, which does not match the intended "use the existing Tailscale daemon like `tailscale serve`" model and introduces an extra node identity for a workflow that should stay attached to the host already running capture. This is the right time to correct it because the remote web feature is new, the capability model is still narrow, and the packet-analysis workflow itself does not need to change.

## What Changes

- Replace the current tailnet web runtime so `thresher analyze web --web-access tailnet` uses the existing local `tailscaled` instance and Serve configuration instead of creating a separate `tsnet` node.
- Keep the analyze web server itself bound to localhost, and publish it remotely through Tailscale Serve on the host's existing tailnet identity.
- Preserve the single remote permission surface using `lbrlabs.com/cap/thresher`, but shift remote authorization to the Serve-backed model and forwarded application capability data instead of direct socket ownership by `tsnet`.
- Require Serve registration and cleanup to merge with any existing host Serve configuration rather than replacing unrelated routes.
- Update the reported remote URL and lifecycle semantics so the printed address reflects the host's existing tailnet identity and the remote route is cleaned up when the analysis session ends.
- **BREAKING**: tailnet web access will no longer appear as a separate Thresher-managed node on the tailnet; it will be exposed from the host's existing Tailscale identity through Serve.

## Non-goals

- Changing packet capture, wrapper decoding, or emitted fields such as `path_id`, `snat`, `dnat`, `inner`, or `disco_meta`.
- Changing the Aperture request flow, batching limits, model selection, or local-only web mode behavior.
- Redesigning the web UI itself beyond any routing or base-path changes required to work behind Tailscale Serve.
- Broadening the capability model beyond the single `lbrlabs.com/cap/thresher` permission surface.

## Capabilities

### New Capabilities

<!-- None. -->

### Modified Capabilities

- `aperture-analysis`: change tailnet web mode so the existing `analyze web --web-access tailnet` command exposes the session through the host's existing Tailscale identity instead of a separate `tsnet` node.
- `analysis-web-ui`: change the remote web entry contract to run behind a Serve-published route while preserving the same live session controls and browser workflow.
- `tsnet-remote-web-access`: replace the `tsnet.Server`-backed remote exposure contract with an existing-`tailscaled`, Serve-backed remote exposure contract while keeping the single-capability gating requirement.

## Impact

- Affected code will include analyze web runtime setup, remote authorization, route generation, startup or shutdown lifecycle, and related command/docs coverage.
- The implementation will shift from `tailscale.com/tsnet` ownership to LocalAPI-driven Serve configuration via the existing local Tailscale daemon, including safe merge and cleanup behavior for shared host Serve config.
- Remote-access tests and documentation will need to be updated to reflect Serve-backed routing and the host-identity URL shape.
- No wire-format or JSON-output changes are required; packet wrapper semantics remain exactly as defined by the existing packet format specification.
