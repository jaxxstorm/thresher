package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

func TestExportJSON(t *testing.T) {
	packets := [][]byte{
		wrapPacket(1, nil, nil, mustIPv4TCPPacket(t)),
		wrapPacket(1, nil, nil, mustIPv4TCPPacket(t)),
		wrapPacket(3, nil, nil, mustIPv6UDPPacket(t)),
		wrapPacket(2, nil, nil, mustIPv4DNSPacket(t)),
		wrapPacket(discoPathID, nil, nil, append(discoMetadata(1), 0xde, 0xad)),
		{1}, // Decode errors are exported as records, not stream errors.
	}
	open := exportTestStream(t, packets...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var originals []Record
	if err := StreamRecords(ctx, open, func(record Record) error {
		originals = append(originals, record)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, includeRaw := range []bool{false, true} {
		t.Run(map[bool]string{false: "omit_raw", true: "include_raw"}[includeRaw], func(t *testing.T) {
			var output bytes.Buffer
			if err := ExportJSON(ctx, &output, open, includeRaw); err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				SchemaVersion int      `json:"schema_version"`
				Packets       []Record `json:"packets"`
				PacketCount   int      `json:"packet_count"`
			}
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, &output)
			}
			if envelope.SchemaVersion != 1 || envelope.PacketCount != len(packets) || len(envelope.Packets) != len(packets) {
				t.Fatalf("unexpected envelope: %+v", envelope)
			}
			lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
			if len(lines) != len(packets)+2 {
				t.Fatalf("expected one line per record, got %d lines", len(lines))
			}
			for i, original := range originals {
				want := original
				if !includeRaw {
					want = recordWithoutRaw(want)
				}
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if line := strings.TrimPrefix(lines[i+1], ","); !json.Valid([]byte(line)) {
					t.Fatalf("record %d is not a single JSON line: %s", i, line)
				}
				gotJSON, err := json.Marshal(envelope.Packets[i])
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotJSON, wantJSON) {
					t.Fatalf("record %d differs:\ngot  %s\nwant %s", i, gotJSON, wantJSON)
				}
			}
			if !includeRaw {
				assertExportNoRaw(t, output.Bytes())
			}
			if envelope.Packets[0].FlowID == "" || envelope.Packets[0].Analysis == nil || envelope.Packets[3].Inner.DNS.TransactionID == "" || envelope.Packets[5].Error == "" {
				t.Fatal("missing analyzer metadata or decode error")
			}
		})
	}
}

func TestExportJSONEmpty(t *testing.T) {
	var output bytes.Buffer
	if err := ExportJSON(context.Background(), &output, exportTestStream(t), false); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "{\"schema_version\":1,\"packets\":[\n],\"packet_count\":0}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRecordWithoutRaw(t *testing.T) {
	original := Record{
		RawHex: "aa", PayloadPreview: "secret", Error: "decode error",
		Analysis: &Analysis{Annotations: []string{"retransmission"}},
		Inner: &Inner{
			RawHex: "bb", PayloadHex: "cc", Protocol: "TCP",
			IP:     &IPMeta{TTL: uint8Ptr(64)},
			TCP:    &TCPMeta{PaddingHex: "dd", PayloadHex: "ee", Seq: uint32Ptr(42), Options: []TCPOptionMeta{{Type: "MSS", Data: "05b4"}}},
			UDP:    &UDPMeta{PayloadHex: "ff", Length: uint16Ptr(8)},
			ICMPv4: &ICMPMeta{PayloadHex: "ab", Type: "echo"},
			ICMPv6: &ICMPMeta{PayloadHex: "cd", Type: "echo"},
			DNS:    &DNSMeta{RawHex: "ef", Questions: []DNSQuestionMeta{{Name: "example.com", Type: "A"}}, Answers: []DNSRecordMeta{{Data: "1.2.3.4"}}},
		},
		DiscoMeta: &DiscoMeta{DERPPub: "key", Frame: &DiscoFrame{TXID: "id", NodeKey: "node", Trailing: "01"}},
	}
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	stripped := recordWithoutRaw(original)
	after, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("original record was mutated")
	}
	encoded, err := json.Marshal(stripped)
	if err != nil {
		t.Fatal(err)
	}
	assertExportNoRaw(t, encoded)
	// Restoring only the omitted fields must recover every original metadata field.
	stripped.RawHex = original.RawHex
	stripped.PayloadPreview = original.PayloadPreview
	stripped.Inner.RawHex = original.Inner.RawHex
	stripped.Inner.PayloadHex = original.Inner.PayloadHex
	stripped.Inner.TCP.PaddingHex = original.Inner.TCP.PaddingHex
	stripped.Inner.TCP.PayloadHex = original.Inner.TCP.PayloadHex
	stripped.Inner.UDP.PayloadHex = original.Inner.UDP.PayloadHex
	stripped.Inner.ICMPv4.PayloadHex = original.Inner.ICMPv4.PayloadHex
	stripped.Inner.ICMPv6.PayloadHex = original.Inner.ICMPv6.PayloadHex
	stripped.Inner.DNS.RawHex = original.Inner.DNS.RawHex
	stripped.DiscoMeta.Frame.Trailing = original.DiscoMeta.Frame.Trailing
	if !reflect.DeepEqual(stripped, original) {
		t.Fatal("decoded metadata changed")
	}
	for _, record := range []Record{{}, {Inner: &Inner{}, DiscoMeta: &DiscoMeta{}}} {
		if got := recordWithoutRaw(record); !reflect.DeepEqual(got, record) {
			t.Fatalf("empty metadata changed: %+v", got)
		}
	}
}

func TestExportJSONErrors(t *testing.T) {
	sentinel := errors.New("test failure")
	t.Run("open", func(t *testing.T) {
		err := ExportJSON(context.Background(), io.Discard, func(context.Context) (io.ReadCloser, error) {
			return nil, sentinel
		}, false)
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v, want opener error", err)
		}
	})
	t.Run("header_read", func(t *testing.T) {
		err := ExportJSON(context.Background(), io.Discard, func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("invalid pcap")), nil
		}, false)
		if err == nil || !strings.Contains(err.Error(), "reading capture stream header") {
			t.Fatalf("got %v, want header error", err)
		}
	})
	t.Run("packet_read", func(t *testing.T) {
		stream, err := exportTestStream(t, []byte{1, 2})(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = ExportJSON(context.Background(), io.Discard, func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data[:len(data)-1])), nil
		}, false)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("got %v, want truncated packet error", err)
		}
	})
	open := exportTestStream(t, []byte{1}, []byte{2})
	for i, stage := range []string{"header", "record", "separator", "second_record", "footer"} {
		t.Run("write_"+stage, func(t *testing.T) {
			calls := 0
			writer := exportWriterFunc(func(p []byte) (int, error) {
				calls++
				if calls == i+1 {
					return 0, sentinel
				}
				return len(p), nil
			})
			if err := ExportJSON(context.Background(), writer, open, false); !errors.Is(err, sentinel) {
				t.Fatalf("got %v, want write error", err)
			}
		})
	}
}

func TestExportJSONCancellation(t *testing.T) {
	t.Run("already_canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := ExportJSON(ctx, io.Discard, exportTestStream(t), false); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		if err := ExportJSON(ctx, io.Discard, exportTestStream(t), false); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want context.DeadlineExceeded", err)
		}
	})
	t.Run("opening", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := ExportJSON(ctx, io.Discard, func(context.Context) (io.ReadCloser, error) {
			cancel()
			return nil, context.Canceled
		}, false)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	})
	t.Run("blocked_read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		opened := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- ExportJSON(ctx, io.Discard, func(ctx context.Context) (io.ReadCloser, error) {
				close(opened)
				return blockingReadCloser{ctx: ctx}, nil
			}, false)
		}()
		<-opened
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want context.Canceled", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("export did not exit after cancellation")
		}
	})
	t.Run("after_record", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var output bytes.Buffer
		writer := exportWriterFunc(func(p []byte) (int, error) {
			if bytes.Contains(p, []byte(`"frame_number"`)) {
				cancel()
			}
			return output.Write(p)
		})
		err := ExportJSON(ctx, writer, exportTestStream(t, []byte{1}), false)
		if !errors.Is(err, context.Canceled) || strings.Contains(output.String(), "packet_count") {
			t.Fatalf("got error %v and output %q, want canceled partial document", err, &output)
		}
	})
}

func exportTestStream(t *testing.T, packets ...[]byte) StreamOpener {
	t.Helper()
	var buffer bytes.Buffer
	writer := pcapgo.NewWriter(&buffer)
	if err := writer.WriteFileHeader(65535, layers.LinkType(147)); err != nil {
		t.Fatal(err)
	}
	for i, packet := range packets {
		if err := writer.WritePacket(gopacket.CaptureInfo{Timestamp: time.Unix(int64(i+1), 0), CaptureLength: len(packet), Length: len(packet)}, packet); err != nil {
			t.Fatal(err)
		}
	}
	return func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buffer.Bytes())), nil
	}
}

func assertExportNoRaw(t *testing.T, data []byte) {
	t.Helper()
	for _, field := range []string{"raw_hex", "payload_hex", "padding_hex", "trailing", "payload_preview"} {
		if bytes.Contains(data, []byte(`"`+field+`":`)) {
			t.Errorf("raw field %q present in JSON", field)
		}
	}
}

type exportWriterFunc func([]byte) (int, error)

func (f exportWriterFunc) Write(p []byte) (int, error) { return f(p) }
