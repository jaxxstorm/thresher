## 1. LocalAPI Capture Wiring

- [x] 1.1 Add the required Tailscale LocalAPI dependency to `go.mod`
- [x] 1.2 Create the `capture` cobra command file and register `thresher capture` on the CLI root
- [x] 1.3 Add `-o` / `--output` flag handling that selects stdout or an output file writer
- [x] 1.4 Implement LocalAPI stream setup against the local `tailscaled` instance and return contextual startup errors
- [x] 1.5 Write command-level tests covering output writer selection and LocalAPI startup failures

## 2. Wrapper and DISCO Decoding

- [x] 2.1 Create a decoder package for parsing the 2-byte little-endian `path_id` plus SNAT and DNAT wrapper fields
- [x] 2.2 Implement Go parsing for DISCO metadata: flags byte, 32-byte DERP key area, source port, address length, source address, and payload handoff
- [x] 2.3 Implement Go parsing for DISCO frame types 1-7, including Ping and Pong-specific fields
- [x] 2.4 Add unit tests with byte fixtures covering valid wrapper parsing, DISCO Ping/Pong decoding, and malformed length handling

## 3. Inner Packet Decoding and JSONL Emission

- [x] 3.1 Implement non-DISCO payload decoding for inner IPv4/IPv6 packets and transport metadata using Go packet libraries
- [x] 3.2 Define the JSONL output struct so each packet includes `path`, `path_id`, `snat`, `dnat`, `disco`, and either `inner` or `disco_meta`
- [x] 3.3 Implement the streaming encoder that writes one JSON object per packet to the chosen writer
- [x] 3.4 Add tests covering TCP/UDP inner packet decoding and JSONL encoding behavior

## 4. End-to-End Capture Behavior

- [x] 4.1 Connect the LocalAPI stream reader to the decoder and JSONL encoder in the `capture` command
- [x] 4.2 Decide and implement per-packet decode error behavior (structured failure record or logged skip) without panicking
- [x] 4.3 Add integration-style tests using mocked capture readers to verify multi-packet streaming behavior
- [x] 4.4 Verify with `go test ./...` and a manual smoke test using `go run . capture` against a running local `tailscaled`
