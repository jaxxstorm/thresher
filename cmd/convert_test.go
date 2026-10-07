package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/jaxxstorm/thresher/internal/capture"
)

func TestConvertCommand(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "capture.pcap")
	data := testPCAP(t, [][]byte{wrappedTCPPacket(t), wrappedBadPacket()})
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newConvertCommand()
	cmd.SetArgs([]string{input})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(filepath.Join(dir, "capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		SchemaVersion int              `json:"schema_version"`
		PacketCount   int              `json:"packet_count"`
		Packets       []map[string]any `json:"packets"`
	}
	if err := json.Unmarshal(output, &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || document.PacketCount != 2 || len(document.Packets) != 2 {
		t.Fatalf("unexpected document: %s", output)
	}
	if document.Packets[0]["stream_id"] == nil || document.Packets[1]["error"] == nil {
		t.Fatalf("missing enrichment or decode error: %s", output)
	}
	if strings.Contains(string(output), "raw_hex") {
		t.Fatalf("raw data in default export: %s", output)
	}
	cmd = newConvertCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{input, "-o", "-", "--include-raw"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(stdout.Bytes()) || !strings.Contains(stdout.String(), "raw_hex") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestConvertFailurePreservesFiles(t *testing.T) {
	for _, mode := range []string{"detailed", "summary"} {
		t.Run(mode, func(t *testing.T) {
			for _, scenario := range []string{"same", "hardlink", "symlink", "invalid", "unsupported", "pcapng", "truncated", "canceled", "publish"} {
				t.Run(scenario, func(t *testing.T) {
					dir := t.TempDir()
					input, output := filepath.Join(dir, "in.pcap"), filepath.Join(dir, "out.json")
					data := testPCAP(t, [][]byte{wrappedTCPPacket(t)})
					switch scenario {
					case "invalid":
						data = []byte("not a pcap")
					case "unsupported":
						data[20] = 1 // Fixture uses little-endian classic PCAP.
					case "truncated":
						data = data[:len(data)-1]
					case "pcapng":
						var buf bytes.Buffer
						writer, err := pcapgo.NewNgWriter(&buf, layers.LinkType(147))
						if err != nil {
							t.Fatal(err)
						}
						if err := writer.Flush(); err != nil {
							t.Fatal(err)
						}
						data = buf.Bytes()
					}
					if err := os.WriteFile(input, data, 0o600); err != nil {
						t.Fatal(err)
					}
					original := []byte("existing output")
					switch scenario {
					case "same":
						output, original = input, data
					case "hardlink":
						if err := os.Link(input, output); err != nil {
							t.Fatal(err)
						}
						original = data
					case "symlink":
						if err := os.Symlink(input, output); err != nil {
							t.Fatal(err)
						}
						original = data
					case "publish":
						if err := os.Mkdir(output, 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(output, "keep"), original, 0o600); err != nil {
							t.Fatal(err)
						}
					default:
						if err := os.WriteFile(output, original, 0o600); err != nil {
							t.Fatal(err)
						}
					}
					cmd := newConvertCommand()
					cmd.SetArgs([]string{input, "-o", output, "--mode", mode})
					if scenario == "canceled" {
						ctx, cancel := context.WithCancel(context.Background())
						cancel()
						cmd.SetContext(ctx)
					}
					err := cmd.Execute()
					if err == nil {
						t.Fatal("expected failure")
					}
					if scenario == "canceled" && !errors.Is(err, context.Canceled) {
						t.Fatalf("expected cancellation, got %v", err)
					}
					if scenario == "same" || scenario == "hardlink" || scenario == "symlink" {
						if !strings.Contains(err.Error(), "different files") {
							t.Fatalf("expected same-file rejection, got %v", err)
						}
					}
					if scenario == "publish" {
						output = filepath.Join(output, "keep")
					}
					got, err := os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, original) {
						t.Fatal("output changed on failure")
					}
					got, err = os.ReadFile(input)
					if err != nil || !bytes.Equal(got, data) {
						t.Fatal("input changed on failure")
					}
					matches, err := filepath.Glob(filepath.Join(dir, ".thresher-convert-*"))
					if err != nil || len(matches) != 0 {
						t.Fatal("temporary output leaked")
					}
				})
			}
		})
	}
}

func TestConvertArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"one", "two"}, {"does-not-exist.pcap"}} {
		cmd := newConvertCommand()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected failure for %v", args)
		}
	}
}

func TestConvertDetailedCompatibility(t *testing.T) {
	data := testPCAP(t, [][]byte{wrappedTCPPacket(t), wrappedBadPacket()})
	input := filepath.Join(t.TempDir(), "capture.pcap")
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []bool{false, true} {
		var want bytes.Buffer
		open := func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
		if err := capture.ExportJSON(context.Background(), &want, open, raw); err != nil {
			t.Fatal(err)
		}
		for _, explicit := range []bool{false, true} {
			cmd := newConvertCommand()
			args := []string{input, "-o", "-"}
			if explicit {
				args = append(args, "--mode", "detailed")
			}
			if raw {
				args = append(args, "--include-raw")
			}
			var got bytes.Buffer
			cmd.SetOut(&got)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatalf("detailed export changed (explicit=%v, raw=%v)", explicit, raw)
			}
		}
	}
}

func TestConvertSummaryDestinations(t *testing.T) {
	for _, destination := range []string{"default", "custom", "existing", "stdout"} {
		t.Run(destination, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "capture.pcap")
			data := testPCAP(t, [][]byte{wrappedTCPPacket(t), wrappedBadPacket()})
			if err := os.WriteFile(input, data, 0o600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "capture.json")
			args := []string{input, "--mode", "summary", "--include-raw=false"}
			switch destination {
			case "custom", "existing":
				output = filepath.Join(dir, "chosen.json")
				args = append(args, "--output", output)
				if destination == "existing" {
					if err := os.WriteFile(output, []byte("replace me"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case "stdout":
				args = append(args, "-o", "-")
			}
			cmd := newConvertCommand()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			got := stdout.Bytes()
			if destination == "stdout" {
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("stdout export created a file: %v", err)
				}
			} else {
				if stdout.Len() != 0 {
					t.Fatalf("unexpected stdout: %s", &stdout)
				}
				var err error
				got, err = os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
			}
			var document map[string]json.RawMessage
			if err := json.Unmarshal(got, &document); err != nil {
				t.Fatal(err)
			}
			if string(document["mode"]) != `"summary"` || string(document["schema_version"]) != "1" || document["packets"] != nil {
				t.Fatalf("not a summary document: %s", got)
			}
			for _, key := range []string{"capture", "endpoints", "conversations", "dns", "findings", "limits", "omissions", "notes"} {
				if document[key] == nil {
					t.Errorf("missing %s", key)
				}
			}
			if strings.Contains(string(got), `"raw_hex"`) || strings.Contains(string(got), `"payload_preview"`) {
				t.Fatalf("raw content in summary: %s", got)
			}
			unchanged, err := os.ReadFile(input)
			if err != nil || !bytes.Equal(unchanged, data) {
				t.Fatal("input changed")
			}
			matches, err := filepath.Glob(filepath.Join(dir, ".thresher-convert-*"))
			if err != nil || len(matches) != 0 {
				t.Fatal("temporary output leaked")
			}
		})
	}
}

func TestConvertInvalidOptionsBeforeIO(t *testing.T) {
	for _, options := range [][]string{
		{"--mode", "invalid"}, {"--mode="}, {"--mode", "Summary"},
		{"--mode", "summary", "--include-raw"}, {"--mode", "summary", "--include-raw=true"},
	} {
		for _, destination := range []string{"existing", "absent", "stdout"} {
			t.Run(strings.Join(options, " ")+"/"+destination, func(t *testing.T) {
				dir := t.TempDir()
				output := filepath.Join(dir, "out.json")
				original := []byte("preserve me")
				if destination == "existing" {
					if err := os.WriteFile(output, original, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if destination == "stdout" {
					output = "-"
				}
				// A missing input ensures option validation precedes input opening.
				args := append([]string{filepath.Join(dir, "missing.pcap"), "-o", output}, options...)
				cmd := newConvertCommand()
				var stdout bytes.Buffer
				cmd.SetOut(&stdout)
				cmd.SetArgs(args)
				err := cmd.Execute()
				want := "must be detailed or summary"
				if len(options) > 2 {
					want = "--include-raw=true is not supported with --mode summary"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("expected option error %q, got %v", want, err)
				}
				if stdout.Len() != 0 {
					t.Fatal("invalid options wrote stdout")
				}
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				if destination == "existing" {
					got, err := os.ReadFile(output)
					if err != nil || !bytes.Equal(got, original) || len(entries) != 1 {
						t.Fatal("invalid options modified output or leaked files")
					}
				} else if len(entries) != 0 {
					t.Fatal("invalid options created files")
				}
			})
		}
	}
}

type convertFailWriter struct{ err error }

func (w convertFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestConvertStdoutWriteFailure(t *testing.T) {
	input := filepath.Join(t.TempDir(), "capture.pcap")
	if err := os.WriteFile(input, testPCAP(t, [][]byte{wrappedTCPPacket(t)}), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("stdout write failed")
	for _, mode := range []string{"detailed", "summary"} {
		cmd := newConvertCommand()
		cmd.SetOut(convertFailWriter{want})
		cmd.SetArgs([]string{input, "--mode", mode, "-o", "-"})
		if err := cmd.Execute(); !errors.Is(err, want) {
			t.Fatalf("%s: expected write error, got %v", mode, err)
		}
	}
}
