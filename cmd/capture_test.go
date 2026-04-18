package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

func TestCaptureCommandStartupFailure(t *testing.T) {
	original := openCaptureStream
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("tailscaled unavailable")
	}
	defer func() { openCaptureStream = original }()

	output, err := executeCommand("capture")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(output, "capture started; waiting for packets") {
		t.Fatalf("expected startup status line, got %q", output)
	}
	if !strings.Contains(err.Error(), "tailscaled unavailable") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCaptureCommandWritesToSelectedFile(t *testing.T) {
	original := openCaptureStream
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(testPCAP(t, [][]byte{wrappedTCPPacket(t)}))), nil
	}
	defer func() { openCaptureStream = original }()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")

	_, err := executeCommand("capture", "-o", path)
	if err != nil {
		t.Fatalf("executeCommand(capture) error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "\"frame_number\":1") {
		t.Fatalf("expected JSONL output, got %q", string(data))
	}
}

func TestCaptureCommandSupportsCompactFormat(t *testing.T) {
	original := openCaptureStream
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(testPCAP(t, [][]byte{wrappedTCPPacket(t)}))), nil
	}
	defer func() { openCaptureStream = original }()

	output, err := executeCommand("capture", "--format", "jsonl-compact")
	if err != nil {
		t.Fatalf("executeCommand(capture) error = %v", err)
	}
	if !strings.Contains(output, "\"number\":1") || !strings.Contains(output, "\"stream_id\":") {
		t.Fatalf("expected compact JSON output, got %q", output)
	}
}

func TestCaptureCommandSupportsSummaryFormat(t *testing.T) {
	original := openCaptureStream
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(testPCAP(t, [][]byte{wrappedTCPPacket(t)}))), nil
	}
	defer func() { openCaptureStream = original }()

	output, err := executeCommand("capture", "--format", "summary")
	if err != nil {
		t.Fatalf("executeCommand(capture) error = %v", err)
	}
	if !strings.Contains(output, "capture started; waiting for packets") || !strings.Contains(output, "TCP") {
		t.Fatalf("expected startup line and summary row, got %q", output)
	}
	if strings.Contains(output, "\"frame_number\":") {
		t.Fatalf("did not expect JSON output in summary mode, got %q", output)
	}
}

func TestCaptureCommandSupportsPacketListFormat(t *testing.T) {
	original := openCaptureStream
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(testPCAP(t, [][]byte{wrappedTCPPacket(t)}))), nil
	}
	defer func() { openCaptureStream = original }()

	output, err := executeCommand("capture", "--format", "packet-list")
	if err != nil {
		t.Fatalf("executeCommand(capture) error = %v", err)
	}
	if !strings.Contains(output, "capture started; waiting for packets") || !strings.Contains(output, "\tTCP\t") {
		t.Fatalf("expected startup line and packet-list row, got %q", output)
	}
	if strings.Contains(output, "\"frame_number\":") {
		t.Fatalf("did not expect JSON output in packet-list mode, got %q", output)
	}
}

func TestCaptureCommandRejectsUnsupportedFormat(t *testing.T) {
	_, err := executeCommand("capture", "--format", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCaptureCommandStreamsMultiplePackets(t *testing.T) {
	original := openCaptureStream
	openCaptureStream = func(context.Context) (io.ReadCloser, error) {
		packets := [][]byte{wrappedTCPPacket(t), wrappedBadPacket()}
		return io.NopCloser(bytes.NewReader(testPCAP(t, packets))), nil
	}
	defer func() { openCaptureStream = original }()

	output, err := executeCommand("capture")
	if err != nil {
		t.Fatalf("executeCommand(capture) error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected startup line plus 2 JSONL records, got %d: %q", len(lines), output)
	}
	if !strings.Contains(lines[0], "capture started; waiting for packets") {
		t.Fatalf("expected startup status line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "\"frame_number\":1") || !strings.Contains(lines[2], "\"frame_number\":2") {
		t.Fatalf("unexpected records %q", output)
	}
	if !strings.Contains(lines[2], "\"error\":") {
		t.Fatalf("expected structured error record, got %q", lines[2])
	}
}

func testPCAP(t *testing.T, packets [][]byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	w := pcapgo.NewWriter(buf)
	if err := w.WriteFileHeader(65535, layers.LinkType(147)); err != nil {
		t.Fatalf("WriteFileHeader() error = %v", err)
	}
	for i, packet := range packets {
		ci := gopacket.CaptureInfo{Timestamp: time.Unix(int64(i+1), 0), CaptureLength: len(packet), Length: len(packet)}
		if err := w.WritePacket(ci, packet); err != nil {
			t.Fatalf("WritePacket() error = %v", err)
		}
	}
	return buf.Bytes()
}

func wrappedTCPPacket(t *testing.T) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP, SrcIP: net.IPv4(100, 64, 0, 1), DstIP: net.IPv4(100, 64, 0, 2)}
	tcp := &layers.TCP{SrcPort: 1234, DstPort: 443, SYN: true}
	_ = tcp.SetNetworkLayerForChecksum(ip)
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, tcp, gopacket.Payload([]byte("hello"))); err != nil {
		t.Fatalf("SerializeLayers() error = %v", err)
	}

	packet := &bytes.Buffer{}
	_ = binary.Write(packet, binary.LittleEndian, uint16(1))
	packet.WriteByte(4)
	packet.Write([]byte{10, 0, 0, 1})
	packet.WriteByte(4)
	packet.Write([]byte{10, 0, 0, 2})
	packet.Write(buf.Bytes())
	return packet.Bytes()
}

func wrappedBadPacket() []byte {
	return []byte{1, 0, 16, 1, 2, 3}
}
