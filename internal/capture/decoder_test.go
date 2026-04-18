package capture

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func TestDecodePacketTCP(t *testing.T) {
	payload := mustIPv4TCPPacket(t)
	data := wrapPacket(1, net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 2), payload)

	record, err := DecodePacket(1, time.Unix(1, 0), data)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}

	if record.Path != "FromPeer" {
		t.Fatalf("unexpected path %q", record.Path)
	}
	if record.PacketOrigin != "captured" {
		t.Fatalf("expected captured packet origin, got %q", record.PacketOrigin)
	}
	if record.Number != 1 || record.Time == "" || record.Length != len(data) {
		t.Fatalf("expected normalized row fields, got %#v", record)
	}
	if record.Src != "100.64.0.1" || record.Dst != "100.64.0.2" || record.Protocol != "TCP" {
		t.Fatalf("unexpected top-level row fields %#v", record)
	}
	if record.FrameLength != len(data) {
		t.Fatalf("unexpected frame length %d", record.FrameLength)
	}
	if record.RawHex == "" {
		t.Fatal("expected raw packet hex")
	}
	if record.Inner == nil || record.Inner.Protocol != "TCP" {
		t.Fatalf("expected TCP inner payload, got %#v", record.Inner)
	}
	if len(record.Inner.Layers) < 2 || record.Inner.Layers[0] != "IPv4" || record.Inner.Layers[1] != "TCP" {
		t.Fatalf("unexpected decoded layers %#v", record.Inner.Layers)
	}
	if record.Inner.SrcPort != 1234 || record.Inner.DstPort != 443 {
		t.Fatalf("unexpected ports: %#v", record.Inner)
	}
	if record.Inner.PayloadLength == nil || *record.Inner.PayloadLength != 5 {
		t.Fatalf("unexpected TCP payload length: %#v", record.Inner)
	}
	if record.Inner.TCP == nil {
		t.Fatalf("expected TCP metadata, got %#v", record.Inner)
	}
	if len(record.Inner.TCP.Flags) != 1 || record.Inner.TCP.Flags[0] != "SYN" {
		t.Fatalf("unexpected TCP flags %#v", record.Inner.TCP.Flags)
	}
	if record.Inner.TCP.Seq == nil || *record.Inner.TCP.Seq != 0 {
		t.Fatalf("unexpected TCP seq %#v", record.Inner.TCP)
	}
	if record.Inner.TCP.Ack == nil || *record.Inner.TCP.Ack != 0 {
		t.Fatalf("unexpected TCP ack %#v", record.Inner.TCP)
	}
	if record.Inner.TCP.Window == nil {
		t.Fatalf("unexpected TCP window %#v", record.Inner.TCP)
	}
	if record.Inner.TCP.DataOffset == nil {
		t.Fatalf("expected TCP data offset %#v", record.Inner.TCP)
	}
	if record.Inner.TCP.Checksum == nil {
		t.Fatalf("expected TCP checksum %#v", record.Inner.TCP)
	}
	if record.Inner.TCP.PayloadHex == "" || record.Inner.PayloadHex == "" {
		t.Fatalf("expected TCP payload hex %#v", record.Inner)
	}
	if record.Inner.IP == nil || record.Inner.IP.TTL == nil || record.Inner.IP.TotalLength == nil {
		t.Fatalf("expected IPv4 metadata %#v", record.Inner.IP)
	}
	if record.Summary == "" {
		t.Fatal("expected packet summary")
	}
	if !strings.Contains(record.Info, "1234 -> 443") {
		t.Fatalf("unexpected packet info %q", record.Info)
	}
}

func TestDecodePacketUDP(t *testing.T) {
	payload := mustIPv6UDPPacket(t)
	data := wrapPacket(3, net.IP{}, net.IP{}, payload)

	record, err := DecodePacket(2, time.Unix(2, 0), data)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}

	if record.Inner == nil || record.Inner.Protocol != "UDP" {
		t.Fatalf("expected UDP inner payload, got %#v", record.Inner)
	}
	if record.Inner.PayloadLength == nil || *record.Inner.PayloadLength != 2 {
		t.Fatalf("unexpected UDP payload length %#v", record.Inner)
	}
	if record.Inner.UDP == nil || record.Inner.UDP.Length == nil || *record.Inner.UDP.Length != 10 {
		t.Fatalf("unexpected UDP metadata %#v", record.Inner)
	}
	if record.Inner.IP == nil || record.Inner.IP.HopLimit == nil || record.Inner.IP.PayloadLength == nil {
		t.Fatalf("expected IPv6 metadata %#v", record.Inner.IP)
	}
	if record.Inner.UDP.PayloadHex == "" {
		t.Fatalf("expected UDP payload hex %#v", record.Inner.UDP)
	}
	if record.Protocol != "UDP" || record.Src != "2001:db8::10" || record.Dst != "2001:db8::20" {
		t.Fatalf("unexpected top-level UDP fields %#v", record)
	}
	if record.PacketOrigin != "synthesized" {
		t.Fatalf("expected synthesized packet origin, got %q", record.PacketOrigin)
	}
}

func TestDecodePacketDNS(t *testing.T) {
	payload := mustIPv4DNSPacket(t)
	data := wrapPacket(2, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), payload)

	record, err := DecodePacket(6, time.Unix(6, 0), data)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}

	if record.Inner == nil || record.Inner.DNS == nil {
		t.Fatalf("expected DNS payload, got %#v", record.Inner)
	}
	if record.Inner.DNS.ID != 0x79a7 {
		t.Fatalf("unexpected DNS id %#x", record.Inner.DNS.ID)
	}
	if !record.Inner.DNS.Response {
		t.Fatal("expected DNS response")
	}
	if len(record.Inner.DNS.Questions) != 1 || record.Inner.DNS.Questions[0].Name != "warpianwzlfqdq.dataplane.rudderstack.com" {
		t.Fatalf("unexpected DNS questions %#v", record.Inner.DNS.Questions)
	}
	if len(record.Inner.DNS.Answers) != 1 || record.Inner.DNS.Answers[0].Data != "54.175.92.109" {
		t.Fatalf("unexpected DNS answers %#v", record.Inner.DNS.Answers)
	}
	if record.Inner.DNS.RawHex == "" {
		t.Fatal("expected DNS raw hex")
	}
	if record.Summary == "" || !bytes.Contains([]byte(record.Summary), []byte("Standard query response")) {
		t.Fatalf("unexpected DNS summary %q", record.Summary)
	}
	if !strings.Contains(record.Info, "Standard query response") {
		t.Fatalf("unexpected DNS info %q", record.Info)
	}
}

func TestAnalyzerAssignsFlowIDsAcrossDirections(t *testing.T) {
	analyzer := NewAnalyzer()
	first, err := DecodePacket(1, time.Unix(1, 0), wrapPacket(1, nil, nil, mustIPv4TCPPacketWithSeq(t, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), 1234, 443, 10, 0, true, true, []byte("hello"))))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	second, err := DecodePacket(2, time.Unix(2, 0), wrapPacket(1, nil, nil, mustIPv4TCPPacketWithSeq(t, net.IPv4(100, 64, 0, 2), net.IPv4(100, 64, 0, 1), 443, 1234, 20, 16, false, true, nil)))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	third, err := DecodePacket(3, time.Unix(3, 0), wrapPacket(1, nil, nil, mustIPv4TCPPacketWithSeq(t, net.IPv4(100, 64, 0, 3), net.IPv4(100, 64, 0, 4), 2000, 80, 1, 0, true, false, nil)))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}

	first = analyzer.Analyze(first)
	second = analyzer.Analyze(second)
	third = analyzer.Analyze(third)

	if first.FlowID == "" || second.FlowID == "" {
		t.Fatalf("expected flow IDs on analyzed packets: %#v %#v", first, second)
	}
	if first.FlowID != second.FlowID {
		t.Fatalf("expected matching flow IDs, got %q and %q", first.FlowID, second.FlowID)
	}
	if third.FlowID == first.FlowID {
		t.Fatalf("expected distinct flow for unrelated conversation, got %q", third.FlowID)
	}
}

func TestAnalyzerComputesRelativeTCPValuesAndAnnotations(t *testing.T) {
	analyzer := NewAnalyzer()
	first := mustAnalyzedTCPRecord(t, analyzer, 1, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), 1234, 443, 100, 0, true, true, []byte("abc"))
	second := mustAnalyzedTCPRecord(t, analyzer, 2, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), 1234, 443, 100, 0, false, true, []byte("abc"))
	third := mustAnalyzedTCPRecord(t, analyzer, 3, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), 1234, 443, 110, 0, false, true, []byte("abc"))
	fourth := mustAnalyzedTCPRecord(t, analyzer, 4, net.IPv4(100, 64, 0, 2), net.IPv4(100, 64, 0, 1), 443, 1234, 500, 113, false, true, nil)
	fifth := mustAnalyzedTCPRecord(t, analyzer, 5, net.IPv4(100, 64, 0, 2), net.IPv4(100, 64, 0, 1), 443, 1234, 501, 113, false, true, nil)

	if second.Inner.TCP.RelativeSeq == nil || *second.Inner.TCP.RelativeSeq != 0 {
		t.Fatalf("expected relative seq 0, got %#v", second.Inner.TCP)
	}
	if !containsString(second.Analysis.Annotations, "retransmission") {
		t.Fatalf("expected retransmission annotation, got %#v", second.Analysis)
	}
	if !containsString(third.Analysis.Annotations, "out_of_order") || !containsString(third.Analysis.Annotations, "unseen_segment") {
		t.Fatalf("expected out-of-order annotations, got %#v", third.Analysis)
	}
	if fourth.Inner.TCP.RelativeAck == nil || *fourth.Inner.TCP.RelativeAck != 13 {
		t.Fatalf("expected relative ack 13, got %#v", fourth.Inner.TCP)
	}
	if fourth.RelativeAck == nil || *fourth.RelativeAck != 13 {
		t.Fatalf("expected promoted relative ack 13, got %#v", fourth)
	}
	if !containsString(fifth.Analysis.Annotations, "duplicate_ack") {
		t.Fatalf("expected duplicate ACK annotation, got %#v", fifth.Analysis)
	}
	if !second.Analysis.Retransmission {
		t.Fatalf("expected explicit retransmission flag, got %#v", second.Analysis)
	}
	if !third.Analysis.OutOfOrder || !third.Analysis.PreviousSegmentNotCaptured {
		t.Fatalf("expected explicit out-of-order flags, got %#v", third.Analysis)
	}
	if !first.Analysis.PartialHistory {
		t.Fatalf("expected initial packet to be marked partial history, got %#v", first.Analysis)
	}
}

func TestAnalyzerCorrelatesDNSTransactions(t *testing.T) {
	analyzer := NewAnalyzer()
	request := mustAnalyzedDNSRecord(t, analyzer, 1, false, 0x79a7, "example.com", nil)
	response := mustAnalyzedDNSRecord(t, analyzer, 2, true, 0x79a7, "example.com", net.IPv4(1, 2, 3, 4))
	unmatched := mustAnalyzedDNSRecord(t, analyzer, 3, true, 0xbeef, "example.net", net.IPv4(5, 6, 7, 8))
	otherFlow := mustAnalyzedDNSRecordOnPorts(t, analyzer, 4, false, 0x79a7, "example.com", nil, 53001, 53)

	if request.Inner.DNS.TransactionID == "" || response.Inner.DNS.TransactionID == "" {
		t.Fatalf("expected DNS transaction IDs, got %#v %#v", request.Inner.DNS, response.Inner.DNS)
	}
	if request.Inner.DNS.TransactionID != response.Inner.DNS.TransactionID {
		t.Fatalf("expected matched transaction IDs, got %q and %q", request.Inner.DNS.TransactionID, response.Inner.DNS.TransactionID)
	}
	if response.Inner.DNS.PeerFrameNumber == nil || *response.Inner.DNS.PeerFrameNumber != 1 {
		t.Fatalf("expected peer frame number 1, got %#v", response.Inner.DNS)
	}
	if response.Inner.DNS.Status != "matched" {
		t.Fatalf("expected matched status, got %#v", response.Inner.DNS)
	}
	if unmatched.Inner.DNS.Status != "unmatched_response" {
		t.Fatalf("expected unmatched response status, got %#v", unmatched.Inner.DNS)
	}
	if otherFlow.Inner.DNS.TransactionID == request.Inner.DNS.TransactionID {
		t.Fatalf("expected repeated DNS IDs on different flows to differ, got %q", otherFlow.Inner.DNS.TransactionID)
	}
}

func TestAnalyzerEvictsOldFlowState(t *testing.T) {
	analyzer := NewAnalyzer()
	for i := 0; i < maxTrackedFlows+1; i++ {
		record := mustAnalyzedTCPRecord(t, analyzer, i+1, net.IPv4(100, 64, 0, byte(i+1)), net.IPv4(100, 64, 1, byte(i+1)), uint16(1000+i), 80, uint32(i), 0, true, false, nil)
		if record.FlowID == "" {
			t.Fatalf("expected flow ID for frame %d", i+1)
		}
	}
	if len(analyzer.flows) != maxTrackedFlows {
		t.Fatalf("expected bounded flow state, got %d", len(analyzer.flows))
	}
}

func TestRecordWritersSupportCompactAndSummary(t *testing.T) {
	record, err := DecodePacket(1, time.Unix(1, 0), wrapPacket(1, nil, nil, mustIPv4TCPPacketWithSeq(t, net.IPv4(100, 113, 191, 35), net.IPv4(100, 91, 175, 29), 63242, 22, 1, 1, false, true, []byte("hello ssh"))))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	record = NewAnalyzer().Analyze(record)

	compactBuf := &bytes.Buffer{}
	compactWriter, err := NewRecordWriter(compactBuf, FormatJSONLCompact)
	if err != nil {
		t.Fatalf("NewRecordWriter() error = %v", err)
	}
	if err := compactWriter.WriteRecord(record); err != nil {
		t.Fatalf("WriteRecord() error = %v", err)
	}
	if !strings.Contains(compactBuf.String(), "\"number\":1") || !strings.Contains(compactBuf.String(), "\"stream_id\":") {
		t.Fatalf("unexpected compact output %q", compactBuf.String())
	}

	summaryBuf := &bytes.Buffer{}
	summaryWriter, err := NewRecordWriter(summaryBuf, FormatSummary)
	if err != nil {
		t.Fatalf("NewRecordWriter() error = %v", err)
	}
	if err := summaryWriter.WriteRecord(record); err != nil {
		t.Fatalf("WriteRecord() error = %v", err)
	}
	if !strings.Contains(summaryBuf.String(), "TCP") || !strings.Contains(summaryBuf.String(), "63242") {
		t.Fatalf("unexpected summary output %q", summaryBuf.String())
	}

	packetListBuf := &bytes.Buffer{}
	packetListWriter, err := NewRecordWriter(packetListBuf, FormatPacketList)
	if err != nil {
		t.Fatalf("NewRecordWriter() error = %v", err)
	}
	if err := packetListWriter.WriteRecord(record); err != nil {
		t.Fatalf("WriteRecord() error = %v", err)
	}
	if !strings.Contains(packetListBuf.String(), "TCP") || !strings.Contains(packetListBuf.String(), "63242") || !strings.Contains(packetListBuf.String(), "\t") {
		t.Fatalf("unexpected packet-list output %q", packetListBuf.String())
	}
}

func TestAnalyzerSetsZeroWindowFlag(t *testing.T) {
	analyzer := NewAnalyzer()
	record, err := DecodePacket(1, time.Unix(1, 0), wrapPacket(1, nil, nil, mustIPv4TCPPacketWithWindow(t, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), 1234, 443, 10, 0, 0, false, true, nil)))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	record = analyzer.Analyze(record)
	if !record.Analysis.ZeroWindow {
		t.Fatalf("expected zero window flag, got %#v", record.Analysis)
	}
}

func TestDecodePacketDiscoPing(t *testing.T) {
	data := wrapPacket(discoPathID, nil, nil, discoMetadata(1))

	record, err := DecodePacket(3, time.Unix(3, 0), data)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}

	if !record.Disco || record.DiscoMeta == nil || record.DiscoMeta.Frame == nil {
		t.Fatalf("expected disco metadata, got %#v", record)
	}
	if record.DiscoMeta.Frame.Type != "Ping" {
		t.Fatalf("unexpected disco type %q", record.DiscoMeta.Frame.Type)
	}
	if record.Protocol != "TSMP/DISCO" || !strings.Contains(record.Info, "Ping") {
		t.Fatalf("unexpected disco row fields %#v", record)
	}
	if record.Dst != "" {
		t.Fatalf("expected no derivable disco dst for ping, got %q", record.Dst)
	}
}

func TestDecodePacketDiscoPong(t *testing.T) {
	data := wrapPacket(discoPathID, nil, nil, discoMetadata(2))

	record, err := DecodePacket(4, time.Unix(4, 0), data)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}

	if record.DiscoMeta == nil || record.DiscoMeta.Frame == nil || record.DiscoMeta.Frame.Type != "Pong" {
		t.Fatalf("unexpected disco frame %#v", record.DiscoMeta)
	}
	if record.DiscoMeta.Frame.PongSrcPort != 41641 {
		t.Fatalf("unexpected pong source port %d", record.DiscoMeta.Frame.PongSrcPort)
	}
	if record.Dst != "2001:db8::2:41641" && record.Dst != "2001:db8::2" {
		t.Fatalf("expected derived disco dst, got %q", record.Dst)
	}
}

func TestDecodePacketMalformedLengths(t *testing.T) {
	_, err := DecodePacket(5, time.Unix(5, 0), []byte{1, 0, 4, 1, 2})
	if err == nil {
		t.Fatal("expected error for malformed wrapper")
	}
}

func TestEncoderWritesJSONL(t *testing.T) {
	buf := &bytes.Buffer{}
	enc := NewEncoder(buf)

	err := enc.Encode(Record{FrameNumber: 1, Timestamp: time.Unix(1, 0), Path: "FromPeer"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var record Record
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record.FrameNumber != 1 {
		t.Fatalf("unexpected frame number %d", record.FrameNumber)
	}
}

func TestStreamJSONLReturnsCleanlyOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- StreamJSONL(ctx, io.Discard, func(ctx context.Context) (io.ReadCloser, error) {
			return blockingReadCloser{ctx: ctx}, nil
		}, FormatJSONL)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamJSONL did not exit after cancellation")
	}
}

func wrapPacket(pathID uint16, snat, dnat net.IP, payload []byte) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, pathID)
	writeIP(buf, snat)
	writeIP(buf, dnat)
	buf.Write(payload)
	return buf.Bytes()
}

func writeIP(buf *bytes.Buffer, ip net.IP) {
	if len(ip) == 0 {
		buf.WriteByte(0)
		return
	}
	ip = normalizeIP(ip)
	buf.WriteByte(byte(len(ip)))
	buf.Write(ip)
}

func normalizeIP(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip.To16()
}

func discoMetadata(messageType byte) []byte {
	buf := &bytes.Buffer{}
	buf.WriteByte(1)
	buf.Write(bytes.Repeat([]byte{0xaa}, 32))
	_ = binary.Write(buf, binary.LittleEndian, uint16(41641))
	addr := net.ParseIP("2001:db8::1").To16()
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(addr)))
	buf.Write(addr)
	frame := discoFrame(messageType)
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(frame)))
	buf.Write(frame)
	return buf.Bytes()
}

func discoFrame(messageType byte) []byte {
	buf := &bytes.Buffer{}
	buf.WriteByte(messageType)
	buf.WriteByte(1)
	buf.Write(bytes.Repeat([]byte{0xbb}, 12))
	if messageType == 1 {
		buf.Write(bytes.Repeat([]byte{0xcc}, 32))
	}
	if messageType == 2 {
		buf.Write(net.ParseIP("2001:db8::2").To16())
		_ = binary.Write(buf, binary.LittleEndian, uint16(41641))
	}
	return buf.Bytes()
}

func mustIPv4TCPPacket(t *testing.T) []byte {
	t.Helper()
	return mustIPv4TCPPacketWithSeq(t, net.IPv4(100, 64, 0, 1), net.IPv4(100, 64, 0, 2), 1234, 443, 0, 0, true, false, []byte("hello"))
}

func mustIPv4TCPPacketWithSeq(t *testing.T, srcIP, dstIP net.IP, srcPort, dstPort uint16, seq, ack uint32, syn, ackFlag bool, payload []byte) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP, SrcIP: srcIP, DstIP: dstIP}
	tcp := &layers.TCP{SrcPort: layers.TCPPort(srcPort), DstPort: layers.TCPPort(dstPort), Seq: seq, Ack: ack, SYN: syn, ACK: ackFlag}
	return mustSerializedTCPPacket(t, buf, ip, tcp, payload)
}

func mustIPv4TCPPacketWithWindow(t *testing.T, srcIP, dstIP net.IP, srcPort, dstPort uint16, seq, ack uint32, window uint16, syn, ackFlag bool, payload []byte) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP, SrcIP: srcIP, DstIP: dstIP}
	tcp := &layers.TCP{SrcPort: layers.TCPPort(srcPort), DstPort: layers.TCPPort(dstPort), Seq: seq, Ack: ack, Window: window, SYN: syn, ACK: ackFlag}
	return mustSerializedTCPPacket(t, buf, ip, tcp, payload)
}

func mustSerializedTCPPacket(t *testing.T, buf gopacket.SerializeBuffer, ip *layers.IPv4, tcp *layers.TCP, payload []byte) []byte {
	t.Helper()
	_ = tcp.SetNetworkLayerForChecksum(ip)
	layersToSerialize := []gopacket.SerializableLayer{ip, tcp}
	if payload != nil {
		layersToSerialize = append(layersToSerialize, gopacket.Payload(payload))
	}
	err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, layersToSerialize...)
	if err != nil {
		t.Fatalf("SerializeLayers() error = %v", err)
	}
	return buf.Bytes()
}

func mustIPv6UDPPacket(t *testing.T) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	ip := &layers.IPv6{Version: 6, HopLimit: 64, NextHeader: layers.IPProtocolUDP, SrcIP: net.ParseIP("2001:db8::10"), DstIP: net.ParseIP("2001:db8::20")}
	udp := &layers.UDP{SrcPort: 5353, DstPort: 41641}
	_ = udp.SetNetworkLayerForChecksum(ip)
	err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, udp, gopacket.Payload([]byte("hi")))
	if err != nil {
		t.Fatalf("SerializeLayers() error = %v", err)
	}
	return buf.Bytes()
}

func mustIPv4DNSPacket(t *testing.T) []byte {
	t.Helper()
	return mustIPv4DNSPacketOnPorts(t, true, 0x79a7, "warpianwzlfqdq.dataplane.rudderstack.com", net.IPv4(54, 175, 92, 109), 53, 53000)
}

func mustIPv4DNSPacketOnPorts(t *testing.T, response bool, id uint16, name string, answerIP net.IP, srcPort, dstPort uint16) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: net.IPv4(100, 100, 100, 100), DstIP: net.IPv4(100, 91, 175, 29)}
	udp := &layers.UDP{SrcPort: layers.UDPPort(srcPort), DstPort: layers.UDPPort(dstPort)}
	_ = udp.SetNetworkLayerForChecksum(ip)
	dns := &layers.DNS{
		ID:           id,
		QR:           response,
		OpCode:       layers.DNSOpCodeQuery,
		ResponseCode: layers.DNSResponseCodeNoErr,
		Questions: []layers.DNSQuestion{{
			Name:  []byte(name),
			Type:  layers.DNSTypeA,
			Class: layers.DNSClassIN,
		}},
	}
	if response && answerIP != nil {
		dns.Answers = []layers.DNSResourceRecord{{
			Name:  []byte(name),
			Type:  layers.DNSTypeA,
			Class: layers.DNSClassIN,
			TTL:   60,
			IP:    answerIP,
		}}
	}
	err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, udp, dns)
	if err != nil {
		t.Fatalf("SerializeLayers() error = %v", err)
	}
	return buf.Bytes()
}

func mustAnalyzedTCPRecord(t *testing.T, analyzer *Analyzer, frame int, srcIP, dstIP net.IP, srcPort, dstPort uint16, seq, ack uint32, syn, ackFlag bool, payload []byte) Record {
	t.Helper()
	record, err := DecodePacket(frame, time.Unix(int64(frame), 0), wrapPacket(1, nil, nil, mustIPv4TCPPacketWithSeq(t, srcIP, dstIP, srcPort, dstPort, seq, ack, syn, ackFlag, payload)))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	return analyzer.Analyze(record)
}

func mustAnalyzedDNSRecord(t *testing.T, analyzer *Analyzer, frame int, response bool, id uint16, name string, answerIP net.IP) Record {
	t.Helper()
	return mustAnalyzedDNSRecordOnPorts(t, analyzer, frame, response, id, name, answerIP, 53, 53000)
}

func mustAnalyzedDNSRecordOnPorts(t *testing.T, analyzer *Analyzer, frame int, response bool, id uint16, name string, answerIP net.IP, srcPort, dstPort uint16) Record {
	t.Helper()
	record, err := DecodePacket(frame, time.Unix(int64(frame), 0), wrapPacket(1, nil, nil, mustIPv4DNSPacketOnPorts(t, response, id, name, answerIP, srcPort, dstPort)))
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	return analyzer.Analyze(record)
}

type blockingReadCloser struct {
	ctx context.Context
}

func (b blockingReadCloser) Read(p []byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (blockingReadCloser) Close() error { return nil }
