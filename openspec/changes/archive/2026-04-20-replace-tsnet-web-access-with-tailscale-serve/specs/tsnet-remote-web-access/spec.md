## RENAMED Requirements

### FROM: Analyze web sessions can be exposed on the tailnet through tsnet
### TO: Analyze web sessions can be exposed on the tailnet through tailscale serve

## MODIFIED Requirements

### Requirement: Analyze web sessions can be exposed on the tailnet through tailscale serve
The system SHALL allow `thresher analyze web` to run with a tailnet-served HTTP route backed by the host's existing `tailscaled` Serve configuration so the active analysis session can be viewed from another device on the same Tailscale network without creating a separate Thresher-managed node.

#### Scenario: Tailnet web access is started explicitly
- **WHEN** the user starts `thresher analyze web` with the explicit tailnet web-access setting
- **THEN** the command starts the analysis session on a localhost HTTP listener and publishes that listener through the existing local `tailscaled` Serve configuration
- **AND** the command reports the resolved tailnet URL that can be opened from another tailnet device on the host's existing tailnet identity

#### Scenario: Tailnet web access preserves the active session controls
- **WHEN** a user opens the tailnet-served analysis page during an active session
- **THEN** the page shows the same live session state, pause control, model control, and streamed analysis output that are available in local web mode

### Requirement: Tailnet web access is gated by one Thresher capability
The system SHALL authorize remote access to the analyze web UI using the Tailscale capability `lbrlabs.com/cap/thresher`, and it SHALL keep the remote web surface under a single permission endpoint rather than requiring separate capability definitions per route.

#### Scenario: Peer with capability can load the remote web session
- **WHEN** a tailnet peer requests the remote analysis web UI and its forwarded Tailscale application capability data includes `lbrlabs.com/cap/thresher`
- **THEN** the request is allowed
- **AND** the peer can use the page and its supporting live-update and control routes for that session

#### Scenario: Peer without capability is denied
- **WHEN** a tailnet peer requests the remote analysis web UI and its forwarded Tailscale application capability data does not include `lbrlabs.com/cap/thresher`
- **THEN** the request is rejected
- **AND** the session remains inaccessible from that peer

#### Scenario: Remote routes stay within one permission surface
- **WHEN** the remote analyze web UI serves the page, snapshot feed, event stream, and control actions
- **THEN** those routes remain under one shared path surface for capability purposes
- **AND** the capability contract does not require separate permission endpoints for each individual route

## ADDED Requirements

### Requirement: Serve-backed tailnet access preserves unrelated host Serve config
The system SHALL merge Thresher's remote web route into the existing host Serve configuration without overwriting unrelated Serve routes.

#### Scenario: Existing Serve routes remain intact
- **WHEN** Thresher enables or disables tailnet web access on a host that already has unrelated Serve configuration
- **THEN** it adds or removes only the dedicated Thresher-owned route
- **AND** unrelated Serve routes remain unchanged
