package capture

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

func TestExportSummaryJSON(t *testing.T) {
	open := exportTestStream(t,
		wrapPacket(1, nil, nil, mustIPv4TCPPacket(t)),
		wrapPacket(1, nil, nil, mustIPv4DNSPacket(t)),
		wrapPacket(254, nil, nil, discoMetadata(1)), []byte{1})
	var first, second bytes.Buffer
	for _, output := range []*bytes.Buffer{&first, &second} {
		if err := ExportSummaryJSON(context.Background(), output, open); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("nondeterministic summary output")
	}
	var d summaryDocument
	if err := json.Unmarshal(first.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.SchemaVersion != 1 || d.Mode != "summary" || d.Capture.PacketCount != 4 || d.Capture.DecodeErrors != 1 || d.Capture.DiscoPackets != 1 || len(d.Conversations) != 2 || d.DNS.ResponsePackets != 1 {
		t.Fatalf("incorrect summary: %+v", d)
	}
	assertExportNoRaw(t, first.Bytes())
}

func TestExportSummaryEmpty(t *testing.T) {
	var out bytes.Buffer
	if err := ExportSummaryJSON(context.Background(), &out, exportTestStream(t)); err != nil {
		t.Fatal(err)
	}
	var d summaryDocument
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Capture.PacketCount != 0 || d.Capture.CapturedBytes != 0 || d.Capture.DurationMS != 0 || d.Capture.Start != nil || d.Capture.End != nil {
		t.Fatalf("nonempty totals: %+v", d.Capture)
	}
	for _, field := range []string{"endpoints", "conversations", "findings", "query_examples", "response_examples"} {
		if !strings.Contains(out.String(), `"`+field+`":[]`) {
			t.Errorf("%s is not an empty array: %s", field, out.String())
		}
	}
	if strings.Contains(out.String(), `"start":`) || strings.Contains(out.String(), `"end":`) || strings.Contains(out.String(), "null") {
		t.Fatalf("unexpected empty fields: %s", out.String())
	}
}

type cancelAtEOFStream struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (s cancelAtEOFStream) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	if err == io.EOF {
		s.cancel()
		return n, context.Canceled
	}
	return n, err
}

func TestStreamSummaryJSON(t *testing.T) {
	open := exportTestStream(t, wrapPacket(1, nil, nil, mustIPv4TCPPacket(t)), []byte{1})
	var expected bytes.Buffer
	if err := ExportSummaryJSON(context.Background(), &expected, open); err != nil {
		t.Fatal(err)
	}
	for _, cancelLive := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel_%t", cancelLive), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			liveOpen := open
			if cancelLive {
				stream, err := open(ctx)
				if err != nil {
					t.Fatal(err)
				}
				liveOpen = func(context.Context) (io.ReadCloser, error) {
					return cancelAtEOFStream{ReadCloser: stream, cancel: cancel}, nil
				}
			}
			var output bytes.Buffer
			if err := StreamSummaryJSON(ctx, &output, liveOpen); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(output.Bytes(), expected.Bytes()) {
				t.Fatalf("live summary differs from offline summary: %s", output.String())
			}
		})
	}
	var output bytes.Buffer
	want := errors.New("capture unavailable")
	err := StreamSummaryJSON(context.Background(), &output, func(context.Context) (io.ReadCloser, error) {
		return nil, want
	})
	if !errors.Is(err, want) || output.Len() != 0 {
		t.Fatalf("startup failure: err=%v output=%s", err, output.String())
	}
}

func TestExportSummaryGzip(t *testing.T) {
	compress := func(data []byte) []byte {
		t.Helper()
		var out bytes.Buffer
		w := gzip.NewWriter(&out)
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_%t", empty), func(t *testing.T) {
			var packets [][]byte
			if !empty {
				packets = [][]byte{wrapPacket(1, nil, nil, mustIPv4TCPPacket(t)), {1}}
			}
			open := exportTestStream(t, packets...)
			stream, err := open(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(stream)
			stream.Close()
			if err != nil {
				t.Fatal(err)
			}
			compressed := compress(data)
			var plain bytes.Buffer
			if err := ExportSummaryJSON(context.Background(), &plain, open); err != nil {
				t.Fatal(err)
			}
			corrupt := bytes.Clone(compressed)
			corrupt[len(corrupt)-8] ^= 1 // Corrupt the gzip CRC, not the PCAP data.
			cases := []struct {
				name string
				data []byte
				want error
			}{
				{"valid", compressed, nil},
				{"truncated_header", compressed[:5], io.ErrUnexpectedEOF},
				{"truncated_body", compressed[:len(compressed)/2], io.ErrUnexpectedEOF},
				{"truncated_trailer", compressed[:len(compressed)-1], io.ErrUnexpectedEOF},
				{"missing_trailer", compressed[:len(compressed)-8], io.ErrUnexpectedEOF},
				{"bad_checksum", corrupt, gzip.ErrChecksum},
			}
			if !empty {
				cases = append(cases, struct {
					name string
					data []byte
					want error
				}{"truncated_pcap", compress(data[:len(data)-1]), io.ErrUnexpectedEOF})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var out bytes.Buffer
					closed := 0
					open := func(context.Context) (io.ReadCloser, error) {
						reader := bytes.NewReader(tc.data)
						return summaryReadCloser{read: reader.Read, close: func() error { closed++; return nil }}, nil
					}
					err := ExportSummaryJSON(context.Background(), &out, open)
					if !errors.Is(err, tc.want) || closed != 1 {
						t.Fatalf("error=%v want=%v closes=%d", err, tc.want, closed)
					}
					if tc.want != nil {
						if out.Len() != 0 {
							t.Fatal("failed gzip scan emitted output")
						}
					} else {
						if !bytes.Equal(out.Bytes(), plain.Bytes()) {
							t.Fatal("gzip summary differs from uncompressed summary")
						}
						if err := ExportJSON(context.Background(), io.Discard, open, false); err != nil {
							t.Fatalf("detailed gzip compatibility: %v", err)
						}
					}
				})
			}
			for _, prefixLength := range []int{2, len(compressed) - 8} {
				t.Run(fmt.Sprintf("cancel_after_%d_bytes", prefixLength), func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					ready, closed := make(chan struct{}), make(chan struct{})
					var once sync.Once
					var out bytes.Buffer
					done := make(chan error, 1)
					go func() {
						reader := bytes.NewReader(compressed[:prefixLength])
						done <- ExportSummaryJSON(ctx, &out, func(context.Context) (io.ReadCloser, error) {
							return summaryReadCloser{read: func(p []byte) (int, error) {
								if reader.Len() > 0 {
									return reader.Read(p)
								}
								close(ready)
								<-closed
								return 0, io.ErrClosedPipe
							}, close: func() error { once.Do(func() { close(closed) }); return nil }}, nil
						})
					}()
					<-ready
					cancel()
					select {
					case err := <-done:
						if !errors.Is(err, context.Canceled) || out.Len() != 0 {
							t.Fatalf("gzip cancellation: error=%v output=%q", err, out.String())
						}
					case <-time.After(2 * time.Second):
						t.Fatal("gzip cancellation did not close original stream")
					}
				})
			}
		})
	}
}

func TestExportSummaryRelativeTimes(t *testing.T) {
	var capture, out bytes.Buffer
	w := pcapgo.NewWriterNanos(&capture)
	if err := w.WriteFileHeader(65535, layers.LinkType(147)); err != nil {
		t.Fatal(err)
	}
	packet := wrapPacket(1, nil, nil, mustIPv4TCPPacket(t))
	for _, stamp := range []time.Time{time.Unix(100, 750000), time.Unix(100, 125000)} {
		if err := w.WritePacket(gopacket.CaptureInfo{Timestamp: stamp, CaptureLength: len(packet), Length: len(packet)}, packet); err != nil {
			t.Fatal(err)
		}
	}
	if err := ExportSummaryJSON(context.Background(), &out, func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(capture.Bytes())), nil
	}); err != nil {
		t.Fatal(err)
	}
	var d summaryDocument
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Capture.DurationMS != 0.625 || d.Conversations[0].StartMS != 0 || d.Conversations[0].EndMS != 0.625 {
		t.Fatalf("relative timing: %+v", d)
	}
	for _, f := range d.Findings {
		for _, e := range f.Examples {
			want := 0.0
			if e.Frame == 1 {
				want = 0.625
			}
			if e.TimeMS != want {
				t.Fatalf("evidence time: %+v", e)
			}
		}
	}
}

func TestExportSummaryFailures(t *testing.T) {
	sentinel := errors.New("summary test failure")
	valid, err := exportTestStream(t, []byte{1})(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(valid)
	valid.Close()
	if err != nil {
		t.Fatal(err)
	}
	for name, open := range map[string]StreamOpener{
		"open":                          func(context.Context) (io.ReadCloser, error) { return nil, sentinel },
		"open_canceled_without_context": func(context.Context) (io.ReadCloser, error) { return nil, context.Canceled },
		"read": func(context.Context) (io.ReadCloser, error) {
			return summaryReadCloser{read: func([]byte) (int, error) { return 0, sentinel }}, nil
		},
		"read_canceled_without_context": func(context.Context) (io.ReadCloser, error) {
			return summaryReadCloser{read: func([]byte) (int, error) { return 0, context.Canceled }}, nil
		},
		"truncated": func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data[:len(data)-1])), nil
		},
		"invalid_header": func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("not a pcap")), nil
		},
		"pcapng": func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte{0x0a, 0x0d, 0x0d, 0x0a, 0, 0, 0, 0})), nil
		},
		"unsupported_link": func(context.Context) (io.ReadCloser, error) {
			var b bytes.Buffer
			if err := pcapgo.NewWriter(&b).WriteFileHeader(65535, layers.LinkTypeEthernet); err != nil {
				t.Fatal(err)
			}
			return io.NopCloser(bytes.NewReader(b.Bytes())), nil
		},
		"late_read": func(context.Context) (io.ReadCloser, error) {
			reader := bytes.NewReader(data)
			return summaryReadCloser{read: func(p []byte) (int, error) {
				if reader.Len() == 0 {
					return 0, sentinel
				}
				return reader.Read(p)
			}}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := ExportSummaryJSON(context.Background(), &out, open)
			if err == nil || out.Len() != 0 {
				t.Fatalf("got error %v, output %q", err, out.String())
			}
			if (name == "open" || name == "read" || name == "late_read") && !errors.Is(err, sentinel) {
				t.Fatalf("lost read/open error: %v", err)
			}
			if strings.Contains(name, "canceled") && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if name == "truncated" && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("lost truncation error: %v", err)
			}
		})
	}
	t.Run("write", func(t *testing.T) {
		writer := exportWriterFunc(func(p []byte) (int, error) { return len(p) / 2, sentinel })
		if err := ExportSummaryJSON(context.Background(), writer, exportTestStream(t)); !errors.Is(err, sentinel) {
			t.Fatalf("lost write error: %v", err)
		}
	})
	t.Run("short_write", func(t *testing.T) {
		writer := exportWriterFunc(func(p []byte) (int, error) { return len(p) / 2, nil })
		if err := ExportSummaryJSON(context.Background(), writer, exportTestStream(t)); !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("lost short write: %v", err)
		}
	})
	t.Run("every_truncation_boundary", func(t *testing.T) {
		for length := 0; length < len(data); length++ {
			if length == 24 {
				continue
			} // A complete header alone is a valid empty PCAP.
			var out bytes.Buffer
			err := ExportSummaryJSON(context.Background(), &out, func(context.Context) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(data[:length])), nil
			})
			if err == nil || out.Len() != 0 {
				t.Fatalf("truncation at %d: error=%v output=%q", length, err, out.String())
			}
		}
	})
}

func TestExportSummaryCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		want := context.Canceled
		if deadline {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			want = context.DeadlineExceeded
		} else {
			cancel()
		}
		var out bytes.Buffer
		err := ExportSummaryJSON(ctx, &out, exportTestStream(t))
		cancel()
		if !errors.Is(err, want) || out.Len() != 0 {
			t.Fatalf("pre-canceled: %v, %q", err, out.String())
		}
	}
	t.Run("opening", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var out bytes.Buffer
		err := ExportSummaryJSON(ctx, &out, func(context.Context) (io.ReadCloser, error) { cancel(); return nil, context.Canceled })
		if !errors.Is(err, context.Canceled) || out.Len() != 0 {
			t.Fatalf("opening cancellation: %v", err)
		}
	})
	for _, afterRecord := range []bool{false, true} {
		t.Run(fmt.Sprintf("blocked_read_after_record_%t", afterRecord), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var prefix []byte
			if afterRecord {
				stream, err := exportTestStream(t, []byte{1})(ctx)
				if err != nil {
					t.Fatal(err)
				}
				prefix, err = io.ReadAll(stream)
				stream.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			ready, closed := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var out bytes.Buffer
			done := make(chan error, 1)
			go func() {
				reader := bytes.NewReader(prefix)
				done <- ExportSummaryJSON(ctx, &out, func(context.Context) (io.ReadCloser, error) {
					return summaryReadCloser{read: func(p []byte) (int, error) {
						if reader.Len() > 0 {
							return reader.Read(p)
						}
						close(ready)
						<-closed
						return 0, io.ErrClosedPipe
					}, close: func() error { once.Do(func() { close(closed) }); return nil }}, nil
				})
			}()
			<-ready
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || out.Len() != 0 {
					t.Fatalf("blocked cancellation: %v, %q", err, out.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not close blocked reader")
			}
		})
	}
	t.Run("at_eof", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream, err := exportTestStream(t, []byte{1})(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer stream.Close()
		var out bytes.Buffer
		err = ExportSummaryJSON(ctx, &out, func(context.Context) (io.ReadCloser, error) {
			return summaryReadCloser{read: func(p []byte) (int, error) {
				n, err := stream.Read(p)
				if err == io.EOF {
					cancel()
				}
				return n, err
			}}, nil
		})
		if !errors.Is(err, context.Canceled) || out.Len() != 0 {
			t.Fatalf("EOF cancellation: %v", err)
		}
	})
	t.Run("writing", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := ExportSummaryJSON(ctx, exportWriterFunc(func(p []byte) (int, error) { cancel(); return len(p), nil }), exportTestStream(t))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("write cancellation: %v", err)
		}
	})
}

func TestExportSummaryRoutineFixtureSize(t *testing.T) {
	// 1,000 in-order, payload-bearing TCP observations in one canonical tuple,
	// alternating directions, with increasing sequence/ack numbers and a nonzero window.
	packets := make([][]byte, 1000)
	for n := range packets {
		src, dst, sport, dport := net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), uint16(1234), uint16(443)
		if n%2 == 1 {
			src, dst, sport, dport = dst, src, dport, sport
		}
		packet := mustIPv4TCPPacketWithWindow(t, src, dst, sport, dport, uint32(n/2*5), uint32((n+1)/2*5), 65535, false, true, []byte("hello"))
		packets[n] = wrapPacket(uint16(n%2), nil, nil, packet)
	}
	open := exportTestStream(t, packets...)
	var detailed, summary, repeat bytes.Buffer
	if err := ExportJSON(context.Background(), &detailed, open, false); err != nil {
		t.Fatal(err)
	}
	if err := ExportSummaryJSON(context.Background(), &summary, open); err != nil {
		t.Fatal(err)
	}
	if err := ExportSummaryJSON(context.Background(), &repeat, open); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(summary.Bytes(), repeat.Bytes()) {
		t.Fatal("fixture output is nondeterministic")
	}
	var detail struct {
		PacketCount uint64   `json:"packet_count"`
		Packets     []Record `json:"packets"`
	}
	var d summaryDocument
	if err := json.Unmarshal(detailed.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(summary.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	var total uint64
	for _, p := range detail.Packets {
		total += uint64(p.FrameLength)
	}
	if d.Capture.PacketCount != 1000 || d.Capture.PacketCount != detail.PacketCount || d.Capture.CapturedBytes != total || total != 49000 || len(d.Conversations) != 1 || d.Conversations[0].AToB.Packets != 500 || d.Conversations[0].BToA.Packets != 500 {
		t.Fatalf("fixture totals differ: %+v", d.Capture)
	}
	if summary.Len()*5 > detailed.Len() {
		t.Fatalf("summary %d bytes exceeds 20%% of detailed %d bytes", summary.Len(), detailed.Len())
	}
	t.Logf("1,000-packet fixture: detailed=%d bytes summary=%d bytes (%.3f%% of detailed; %.3f%% reduction), packets=%d captured_bytes=%d", detailed.Len(), summary.Len(), 100*float64(summary.Len())/float64(detailed.Len()), 100*(1-float64(summary.Len())/float64(detailed.Len())), d.Capture.PacketCount, total)
}

func TestSummaryAccumulatorBounds(t *testing.T) {
	s := newSummaryAccumulator()
	for n := 1; n <= 10000; n++ {
		r := summaryTestRecord(n)
		r.Inner.SrcIP, r.Inner.DstIP = fmt.Sprintf("10.%d.%d.1", n/256, n%256), fmt.Sprintf("10.%d.%d.2", n/256, n%256)
		r.PathID = uint16(n)
		r.PacketOrigin = fmt.Sprint(n)
		r.Protocol = fmt.Sprint(n)
		r.Analysis = &Analysis{Annotations: append(append([]string{}, summaryFindingCategories[:]...), fmt.Sprint(n))}
		r.Inner.DNS = &DNSMeta{Response: n%2 == 0, ResponseCode: fmt.Sprint(n), Questions: make([]DNSQuestionMeta, 10), Answers: make([]DNSRecordMeta, 10)}
		s.add(r)
	}
	d := s.finalize()
	if len(s.conversations) != 100 || len(s.endpoints) != 200 || len(d.Conversations) != 100 || len(d.Endpoints) != 200 || len(d.Findings) != len(summaryFindingCategories) || len(d.DNS.Queries) != 20 || len(d.DNS.Responses) != 20 || len(d.Capture.Paths) > 6 || len(d.Capture.Origins) > 3 || len(d.Capture.Protocols) > 7 || len(d.DNS.ResponseCodes) != 1 || d.Capture.PacketCount != 10000 || d.Omissions.ConversationPackets != 9900 || d.Omissions.DNSQueryExamples != 4980 || d.Omissions.DNSResponseExamples != 4980 {
		t.Fatalf("accumulator exceeded caps or lost totals: %+v", d.Omissions)
	}
	for _, f := range d.Findings {
		if len(f.Examples) != 3 || f.Packets != 10000 || d.Omissions.FindingExamples[f.Category] != 9997 {
			t.Fatalf("unbounded finding: %+v", f)
		}
	}
	for _, side := range [][]summaryDNSExample{d.DNS.Queries, d.DNS.Responses} {
		for _, e := range side {
			if len(e.Questions) != 4 || len(e.Answers) != 4 {
				t.Fatalf("unbounded DNS lists: %+v", e)
			}
		}
	}
}

type summaryReadCloser struct {
	read  func([]byte) (int, error)
	close func() error
}

func (r summaryReadCloser) Read(p []byte) (int, error) { return r.read(p) }
func (r summaryReadCloser) Close() error {
	if r.close != nil {
		return r.close()
	}
	return nil
}
