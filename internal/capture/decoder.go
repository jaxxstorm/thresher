package capture

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const discoPathID = 254

var pathNames = map[uint16]string{
	0:   "FromLocal",
	1:   "FromPeer",
	2:   "Synthesized (Inbound / ToLocal)",
	3:   "Synthesized (Outbound / ToPeer)",
	254: "Disco frame",
}

var discoTypeNames = map[uint8]string{
	1: "Ping",
	2: "Pong",
	3: "Call me maybe",
	4: "Bind UDP Relay Endpoint",
	5: "Bind UDP Relay Endpoint Challenge",
	6: "Bind UDP Relay Endpoint Answer",
	7: "Call me maybe via",
}

type Record struct {
	FrameNumber               int        `json:"frame_number"`
	Timestamp                 time.Time  `json:"timestamp"`
	FrameLength               int        `json:"frame_length"`
	Number                    int        `json:"number"`
	Time                      string     `json:"time,omitempty"`
	Src                       string     `json:"src,omitempty"`
	Dst                       string     `json:"dst,omitempty"`
	Protocol                  string     `json:"protocol,omitempty"`
	Length                    int        `json:"length"`
	Info                      string     `json:"info,omitempty"`
	PayloadLength             *int       `json:"payload_length,omitempty"`
	PayloadPreview            string     `json:"payload_preview,omitempty"`
	RelativeSeq               *uint32    `json:"relative_seq,omitempty"`
	RelativeAck               *uint32    `json:"relative_ack,omitempty"`
	Path                      string     `json:"path,omitempty"`
	PathID                    uint16     `json:"path_id"`
	PacketOrigin              string     `json:"packet_origin,omitempty"`
	SNAT                      string     `json:"snat,omitempty"`
	DNAT                      string     `json:"dnat,omitempty"`
	Disco                     bool       `json:"disco"`
	StreamID                  string     `json:"stream_id,omitempty"`
	ConversationKey           string     `json:"conversation_key,omitempty"`
	StreamPacketNumber        *int       `json:"stream_packet_number,omitempty"`
	TimeSinceStreamStart      *float64   `json:"time_since_stream_start,omitempty"`
	TimeSincePreviousInStream *float64   `json:"time_since_previous_in_stream,omitempty"`
	TransportDirection        string     `json:"transport_direction,omitempty"`
	FlowID                    string     `json:"flow_id,omitempty"`
	FlowDir                   string     `json:"flow_direction,omitempty"`
	Summary                   string     `json:"summary,omitempty"`
	RawHex                    string     `json:"raw_hex,omitempty"`
	Inner                     *Inner     `json:"inner,omitempty"`
	DiscoMeta                 *DiscoMeta `json:"disco_meta,omitempty"`
	Analysis                  *Analysis  `json:"analysis,omitempty"`
	Error                     string     `json:"error,omitempty"`
}

type Analysis struct {
	Annotations                []string `json:"annotations,omitempty"`
	Notes                      []string `json:"notes,omitempty"`
	PartialHistory             bool     `json:"partial_history,omitempty"`
	Retransmission             bool     `json:"retransmission,omitempty"`
	FastRetransmission         bool     `json:"fast_retransmission,omitempty"`
	OutOfOrder                 bool     `json:"out_of_order,omitempty"`
	PreviousSegmentNotCaptured bool     `json:"previous_segment_not_captured,omitempty"`
	ZeroWindow                 bool     `json:"zero_window,omitempty"`
}

type Inner struct {
	IPVersion     int       `json:"ip_version"`
	Layers        []string  `json:"layers,omitempty"`
	Protocol      string    `json:"protocol"`
	SrcIP         string    `json:"src_ip"`
	DstIP         string    `json:"dst_ip"`
	SrcPort       uint16    `json:"src_port,omitempty"`
	DstPort       uint16    `json:"dst_port,omitempty"`
	PayloadLength *int      `json:"payload_length,omitempty"`
	RawHex        string    `json:"raw_hex,omitempty"`
	PayloadHex    string    `json:"payload_hex,omitempty"`
	IP            *IPMeta   `json:"ip,omitempty"`
	TCP           *TCPMeta  `json:"tcp,omitempty"`
	UDP           *UDPMeta  `json:"udp,omitempty"`
	ICMPv4        *ICMPMeta `json:"icmpv4,omitempty"`
	ICMPv6        *ICMPMeta `json:"icmpv6,omitempty"`
	DNS           *DNSMeta  `json:"dns,omitempty"`
}

type IPMeta struct {
	TTL            *uint8   `json:"ttl,omitempty"`
	HopLimit       *uint8   `json:"hop_limit,omitempty"`
	HeaderLength   *uint8   `json:"header_length,omitempty"`
	TotalLength    *uint16  `json:"total_length,omitempty"`
	PayloadLength  *uint16  `json:"payload_length,omitempty"`
	TypeOfService  *uint8   `json:"type_of_service,omitempty"`
	TrafficClass   *uint8   `json:"traffic_class,omitempty"`
	FlowLabel      *uint32  `json:"flow_label,omitempty"`
	Identification *uint16  `json:"identification,omitempty"`
	Flags          []string `json:"flags,omitempty"`
	FragmentOffset *uint16  `json:"fragment_offset,omitempty"`
	Checksum       *uint16  `json:"checksum,omitempty"`
	NextHeader     string   `json:"next_header,omitempty"`
}

type TCPMeta struct {
	Flags              []string        `json:"flags,omitempty"`
	Seq                *uint32         `json:"seq,omitempty"`
	Ack                *uint32         `json:"ack,omitempty"`
	RelativeSeq        *uint32         `json:"relative_seq,omitempty"`
	RelativeAck        *uint32         `json:"relative_ack,omitempty"`
	Window             *uint16         `json:"window,omitempty"`
	Checksum           *uint16         `json:"checksum,omitempty"`
	DataOffset         *uint8          `json:"data_offset,omitempty"`
	Urgent             *uint16         `json:"urgent,omitempty"`
	TimestampValue     *uint32         `json:"timestamp_value,omitempty"`
	TimestampEchoReply *uint32         `json:"timestamp_echo_reply,omitempty"`
	Options            []TCPOptionMeta `json:"options,omitempty"`
	PaddingHex         string          `json:"padding_hex,omitempty"`
	PayloadHex         string          `json:"payload_hex,omitempty"`
}

type UDPMeta struct {
	Length     *uint16 `json:"length,omitempty"`
	Checksum   *uint16 `json:"checksum,omitempty"`
	PayloadHex string  `json:"payload_hex,omitempty"`
}

type TCPOptionMeta struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
}

type DNSMeta struct {
	ID                 uint16            `json:"id"`
	Response           bool              `json:"response"`
	OpCode             string            `json:"opcode,omitempty"`
	ResponseCode       string            `json:"response_code,omitempty"`
	TransactionID      string            `json:"transaction_id,omitempty"`
	PeerFrameNumber    *int              `json:"peer_frame_number,omitempty"`
	Status             string            `json:"status,omitempty"`
	ResponseTimeMillis *float64          `json:"response_time_ms,omitempty"`
	Authoritative      bool              `json:"authoritative"`
	Truncated          bool              `json:"truncated"`
	RecursionDesired   bool              `json:"recursion_desired"`
	RecursionAvailable bool              `json:"recursion_available"`
	Questions          []DNSQuestionMeta `json:"questions,omitempty"`
	Answers            []DNSRecordMeta   `json:"answers,omitempty"`
	Authorities        []DNSRecordMeta   `json:"authorities,omitempty"`
	Additionals        []DNSRecordMeta   `json:"additionals,omitempty"`
	RawHex             string            `json:"raw_hex,omitempty"`
}

type ICMPMeta struct {
	Type       string  `json:"type,omitempty"`
	TypeNumber *uint8  `json:"type_number,omitempty"`
	CodeNumber *uint8  `json:"code_number,omitempty"`
	TypeCode   string  `json:"type_code,omitempty"`
	Checksum   *uint16 `json:"checksum,omitempty"`
	Id         *uint16 `json:"id,omitempty"`
	Seq        *uint16 `json:"seq,omitempty"`
	PayloadHex string  `json:"payload_hex,omitempty"`
}

type DNSQuestionMeta struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class,omitempty"`
}

type DNSRecordMeta struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class,omitempty"`
	TTL   uint32 `json:"ttl,omitempty"`
	Data  string `json:"data,omitempty"`
}

type DiscoMeta struct {
	FromDERP bool        `json:"from_derp"`
	DERPPub  string      `json:"derp_pub_key,omitempty"`
	SrcIP    string      `json:"src_ip,omitempty"`
	SrcPort  uint16      `json:"src_port"`
	Frame    *DiscoFrame `json:"frame,omitempty"`
}

type DiscoFrame struct {
	Type        string `json:"type,omitempty"`
	Version     uint8  `json:"version"`
	TXID        string `json:"txid,omitempty"`
	NodeKey     string `json:"node_key,omitempty"`
	PongSrc     string `json:"pong_src,omitempty"`
	PongSrcPort uint16 `json:"pong_src_port,omitempty"`
	Trailing    string `json:"trailing,omitempty"`
}

func DecodePacket(frameNumber int, timestamp time.Time, data []byte) (Record, error) {
	record := Record{FrameNumber: frameNumber, Timestamp: timestamp, FrameLength: len(data), Number: frameNumber, Time: timestamp.Format(time.RFC3339Nano), Length: len(data), RawHex: hex.EncodeToString(data)}

	if len(data) < 2 {
		return record, fmt.Errorf("packet too short for path_id")
	}

	offset := 0
	pathID := binary.LittleEndian.Uint16(data[offset : offset+2])
	record.PathID = pathID
	record.Path = pathNames[pathID]
	record.PacketOrigin = packetOrigin(pathID)
	offset += 2

	snat, next, err := parseAddressField(data, offset, "snat")
	if err != nil {
		return record, err
	}
	record.SNAT = snat
	offset = next

	dnat, next, err := parseAddressField(data, offset, "dnat")
	if err != nil {
		return record, err
	}
	record.DNAT = dnat
	offset = next

	if offset > len(data) {
		return record, fmt.Errorf("packet payload offset exceeds packet length")
	}

	payload := data[offset:]
	record.Disco = pathID == discoPathID

	if record.Disco {
		meta, err := parseDisco(payload)
		if err != nil {
			return record, err
		}
		record.DiscoMeta = meta
		record.Summary = discoSummary(meta)
		populateDiscoColumns(&record)
		return record, nil
	}

	inner, err := parseInnerPacket(payload)
	if err != nil {
		return record, err
	}
	record.Inner = inner
	record.Summary = innerSummary(inner)
	populateInnerColumns(&record)

	return record, nil
}

func RecordDecodeError(frameNumber int, timestamp time.Time, err error) Record {
	return Record{FrameNumber: frameNumber, Timestamp: timestamp, Error: err.Error()}
}

func RecordDecodeErrorFrom(record Record, err error) Record {
	record.Error = err.Error()
	if record.Info == "" {
		record.Info = err.Error()
	}
	return record
}

func parseAddressField(data []byte, offset int, fieldName string) (string, int, error) {
	if offset >= len(data) {
		return "", offset, fmt.Errorf("packet too short for %s length", fieldName)
	}

	addrLen := int(data[offset])
	offset++
	if offset+addrLen > len(data) {
		return "", offset, fmt.Errorf("packet too short for %s bytes", fieldName)
	}

	if addrLen == 0 {
		return "", offset, nil
	}

	addr := net.IP(data[offset : offset+addrLen])
	if addr.String() == "<nil>" {
		return "", offset + addrLen, fmt.Errorf("invalid %s address length %d", fieldName, addrLen)
	}

	return addr.String(), offset + addrLen, nil
}

func parseDisco(data []byte) (*DiscoMeta, error) {
	if len(data) < 37 {
		return nil, fmt.Errorf("packet too short for disco metadata")
	}

	offset := 0
	fromDERP := data[offset]&0x01 != 0
	offset++

	derpPubRaw := data[offset : offset+32]
	offset += 32

	if offset+2 > len(data) {
		return nil, fmt.Errorf("packet too short for disco source port")
	}
	srcPort := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	if offset+2 > len(data) {
		return nil, fmt.Errorf("packet too short for disco address length")
	}
	addrLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+addrLen > len(data) {
		return nil, fmt.Errorf("packet too short for disco source address")
	}

	srcIP := net.IP(data[offset : offset+addrLen]).String()
	offset += addrLen

	if offset+2 > len(data) {
		return nil, fmt.Errorf("packet too short for disco payload length")
	}
	_ = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	frame, err := parseDiscoFrame(data[offset:])
	if err != nil {
		return nil, err
	}

	meta := &DiscoMeta{FromDERP: fromDERP, SrcIP: srcIP, SrcPort: srcPort, Frame: frame}
	if fromDERP {
		meta.DERPPub = hex.EncodeToString(derpPubRaw)
	}

	return meta, nil
}

func parseDiscoFrame(data []byte) (*DiscoFrame, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("packet too short for disco frame")
	}

	offset := 0
	messageType := data[offset]
	offset++
	version := data[offset]
	offset++

	frame := &DiscoFrame{Type: discoTypeNames[messageType], Version: version}

	if messageType == 1 || messageType == 2 {
		if offset+12 > len(data) {
			return nil, fmt.Errorf("packet too short for disco transaction id")
		}
		frame.TXID = hex.EncodeToString(data[offset : offset+12])
		offset += 12
	}

	if messageType == 1 {
		if offset+32 > len(data) {
			return nil, fmt.Errorf("packet too short for disco node key")
		}
		frame.NodeKey = hex.EncodeToString(data[offset : offset+32])
		offset += 32
	}

	if messageType == 2 {
		if offset+16 > len(data) {
			return nil, fmt.Errorf("packet too short for disco pong source")
		}
		frame.PongSrc = net.IP(data[offset : offset+16]).String()
		offset += 16

		if offset+2 > len(data) {
			return nil, fmt.Errorf("packet too short for disco pong source port")
		}
		frame.PongSrcPort = binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
	}

	if offset < len(data) {
		frame.Trailing = hex.EncodeToString(data[offset:])
	}

	return frame, nil
}

func parseInnerPacket(data []byte) (*Inner, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("packet payload is empty")
	}

	version := data[0] >> 4
	var firstLayer gopacket.Decoder
	switch version {
	case 4:
		firstLayer = layers.LayerTypeIPv4
	case 6:
		firstLayer = layers.LayerTypeIPv6
	default:
		return nil, fmt.Errorf("unsupported inner IP version %d", version)
	}

	packet := gopacket.NewPacket(data, firstLayer, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
	if errLayer := packet.ErrorLayer(); errLayer != nil {
		return nil, fmt.Errorf("decoding inner packet: %w", errLayer.Error())
	}

	inner := &Inner{IPVersion: int(version)}
	inner.RawHex = hex.EncodeToString(data)
	inner.Layers = packetLayerNames(packet)

	if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		ipv4 := ipv4Layer.(*layers.IPv4)
		inner.SrcIP = ipv4.SrcIP.String()
		inner.DstIP = ipv4.DstIP.String()
		inner.Protocol = ipv4.Protocol.String()
		inner.IP = &IPMeta{
			TTL:            uint8Ptr(ipv4.TTL),
			HeaderLength:   uint8Ptr(ipv4.IHL * 4),
			TotalLength:    uint16Ptr(ipv4.Length),
			TypeOfService:  uint8Ptr(ipv4.TOS),
			Identification: uint16Ptr(ipv4.Id),
			Flags:          ipv4Flags(ipv4.Flags),
			FragmentOffset: uint16Ptr(ipv4.FragOffset),
			Checksum:       uint16Ptr(ipv4.Checksum),
		}
	}

	if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
		ipv6 := ipv6Layer.(*layers.IPv6)
		inner.SrcIP = ipv6.SrcIP.String()
		inner.DstIP = ipv6.DstIP.String()
		inner.Protocol = ipv6.NextHeader.String()
		inner.IP = &IPMeta{
			HopLimit:      uint8Ptr(ipv6.HopLimit),
			TrafficClass:  uint8Ptr(ipv6.TrafficClass),
			FlowLabel:     uint32Ptr(ipv6.FlowLabel),
			PayloadLength: uint16Ptr(ipv6.Length),
			NextHeader:    ipv6.NextHeader.String(),
		}
	}

	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp := tcpLayer.(*layers.TCP)
		inner.Protocol = "TCP"
		inner.SrcPort = uint16(tcp.SrcPort)
		inner.DstPort = uint16(tcp.DstPort)
		inner.PayloadLength = intPtr(len(tcp.Payload))
		inner.PayloadHex = hex.EncodeToString(tcp.Payload)
		inner.TCP = &TCPMeta{
			Flags:      tcpFlags(tcp),
			Seq:        uint32Ptr(tcp.Seq),
			Ack:        uint32Ptr(tcp.Ack),
			Window:     uint16Ptr(tcp.Window),
			Checksum:   uint16Ptr(tcp.Checksum),
			DataOffset: uint8Ptr(tcp.DataOffset * 4),
			Options:    tcpOptions(tcp.Options),
			PaddingHex: hex.EncodeToString(tcp.Padding),
			PayloadHex: hex.EncodeToString(tcp.Payload),
		}
		if tcp.URG {
			inner.TCP.Urgent = uint16Ptr(tcp.Urgent)
		}
		setTCPTimestamps(inner.TCP, tcp.Options)
	}

	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		inner.Protocol = "UDP"
		inner.SrcPort = uint16(udp.SrcPort)
		inner.DstPort = uint16(udp.DstPort)
		inner.PayloadLength = intPtr(len(udp.Payload))
		inner.PayloadHex = hex.EncodeToString(udp.Payload)
		inner.UDP = &UDPMeta{Length: uint16Ptr(udp.Length), Checksum: uint16Ptr(udp.Checksum), PayloadHex: hex.EncodeToString(udp.Payload)}
	}

	if icmpv4Layer := packet.Layer(layers.LayerTypeICMPv4); icmpv4Layer != nil {
		icmp := icmpv4Layer.(*layers.ICMPv4)
		inner.Protocol = "ICMPv4"
		inner.PayloadLength = intPtr(len(icmp.Payload))
		inner.PayloadHex = hex.EncodeToString(icmp.Payload)
		inner.ICMPv4 = &ICMPMeta{
			Type:       fmt.Sprintf("type_%d", icmp.TypeCode.Type()),
			TypeNumber: uint8Ptr(icmp.TypeCode.Type()),
			CodeNumber: uint8Ptr(icmp.TypeCode.Code()),
			TypeCode:   icmp.TypeCode.String(),
			Checksum:   uint16Ptr(icmp.Checksum),
			Id:         uint16Ptr(icmp.Id),
			Seq:        uint16Ptr(icmp.Seq),
			PayloadHex: hex.EncodeToString(icmp.Payload),
		}
	}

	if icmpv6Layer := packet.Layer(layers.LayerTypeICMPv6); icmpv6Layer != nil {
		icmp := icmpv6Layer.(*layers.ICMPv6)
		inner.Protocol = "ICMPv6"
		inner.PayloadLength = intPtr(len(icmp.Payload))
		inner.PayloadHex = hex.EncodeToString(icmp.Payload)
		inner.ICMPv6 = &ICMPMeta{
			Type:       fmt.Sprintf("type_%d", icmp.TypeCode.Type()),
			TypeNumber: uint8Ptr(icmp.TypeCode.Type()),
			CodeNumber: uint8Ptr(icmp.TypeCode.Code()),
			TypeCode:   icmp.TypeCode.String(),
			Checksum:   uint16Ptr(icmp.Checksum),
			PayloadHex: hex.EncodeToString(icmp.Payload),
		}
	}

	if dnsLayer := packet.Layer(layers.LayerTypeDNS); dnsLayer != nil {
		dns := dnsLayer.(*layers.DNS)
		inner.DNS = parseDNS(dns)
		if inner.PayloadHex == "" {
			inner.PayloadHex = hex.EncodeToString(dns.LayerContents())
		}
	}

	if inner.SrcIP == "" || inner.DstIP == "" {
		return nil, fmt.Errorf("decoded inner packet missing IP addresses")
	}

	return inner, nil
}

func packetLayerNames(packet gopacket.Packet) []string {
	layersSeen := packet.Layers()
	if len(layersSeen) == 0 {
		return nil
	}

	names := make([]string, 0, len(layersSeen))
	for _, layer := range layersSeen {
		names = append(names, layer.LayerType().String())
	}
	return names
}

func ipv4Flags(flags layers.IPv4Flag) []string {
	values := make([]string, 0, 3)
	if flags&layers.IPv4MoreFragments != 0 {
		values = append(values, "MF")
	}
	if flags&layers.IPv4DontFragment != 0 {
		values = append(values, "DF")
	}
	if flags&layers.IPv4EvilBit != 0 {
		values = append(values, "EVIL")
	}
	return values
}

func tcpFlags(tcp *layers.TCP) []string {
	flags := make([]string, 0, 8)
	if tcp.FIN {
		flags = append(flags, "FIN")
	}
	if tcp.SYN {
		flags = append(flags, "SYN")
	}
	if tcp.RST {
		flags = append(flags, "RST")
	}
	if tcp.PSH {
		flags = append(flags, "PSH")
	}
	if tcp.ACK {
		flags = append(flags, "ACK")
	}
	if tcp.URG {
		flags = append(flags, "URG")
	}
	if tcp.ECE {
		flags = append(flags, "ECE")
	}
	if tcp.CWR {
		flags = append(flags, "CWR")
	}
	if tcp.NS {
		flags = append(flags, "NS")
	}
	return flags
}

func setTCPTimestamps(meta *TCPMeta, options []layers.TCPOption) {
	for _, option := range options {
		if option.OptionType != layers.TCPOptionKindTimestamps || len(option.OptionData) < 8 {
			continue
		}

		meta.TimestampValue = uint32Ptr(binary.BigEndian.Uint32(option.OptionData[:4]))
		meta.TimestampEchoReply = uint32Ptr(binary.BigEndian.Uint32(option.OptionData[4:8]))
		return
	}
}

func tcpOptions(options []layers.TCPOption) []TCPOptionMeta {
	if len(options) == 0 {
		return nil
	}

	result := make([]TCPOptionMeta, 0, len(options))
	for _, option := range options {
		meta := TCPOptionMeta{Type: tcpOptionName(option.OptionType)}
		if len(option.OptionData) > 0 {
			meta.Data = hex.EncodeToString(option.OptionData)
		}
		result = append(result, meta)
	}
	return result
}

func tcpOptionName(kind layers.TCPOptionKind) string {
	switch kind {
	case layers.TCPOptionKindEndList:
		return "EOL"
	case layers.TCPOptionKindNop:
		return "NOP"
	case layers.TCPOptionKindMSS:
		return "MSS"
	case layers.TCPOptionKindWindowScale:
		return "WindowScale"
	case layers.TCPOptionKindSACKPermitted:
		return "SACKPermitted"
	case layers.TCPOptionKindSACK:
		return "SACK"
	case layers.TCPOptionKindTimestamps:
		return "Timestamps"
	default:
		return fmt.Sprintf("Unknown(%d)", kind)
	}
}

func parseDNS(dns *layers.DNS) *DNSMeta {
	meta := &DNSMeta{
		ID:                 dns.ID,
		Response:           dns.QR,
		OpCode:             dns.OpCode.String(),
		ResponseCode:       dns.ResponseCode.String(),
		Authoritative:      dns.AA,
		Truncated:          dns.TC,
		RecursionDesired:   dns.RD,
		RecursionAvailable: dns.RA,
		RawHex:             hex.EncodeToString(dns.LayerContents()),
	}

	for _, question := range dns.Questions {
		meta.Questions = append(meta.Questions, DNSQuestionMeta{
			Name:  strings.TrimSuffix(string(question.Name), "."),
			Type:  question.Type.String(),
			Class: question.Class.String(),
		})
	}

	meta.Answers = dnsRecords(dns.Answers)
	meta.Authorities = dnsRecords(dns.Authorities)
	meta.Additionals = dnsRecords(dns.Additionals)
	return meta
}

func dnsRecords(records []layers.DNSResourceRecord) []DNSRecordMeta {
	if len(records) == 0 {
		return nil
	}

	result := make([]DNSRecordMeta, 0, len(records))
	for _, record := range records {
		result = append(result, DNSRecordMeta{
			Name:  strings.TrimSuffix(string(record.Name), "."),
			Type:  record.Type.String(),
			Class: record.Class.String(),
			TTL:   record.TTL,
			Data:  dnsRecordData(record),
		})
	}
	return result
}

func dnsRecordData(record layers.DNSResourceRecord) string {
	switch record.Type {
	case layers.DNSTypeA:
		return record.IP.String()
	case layers.DNSTypeAAAA:
		return record.IP.String()
	case layers.DNSTypeCNAME:
		return strings.TrimSuffix(string(record.CNAME), ".")
	case layers.DNSTypeNS:
		return strings.TrimSuffix(string(record.NS), ".")
	case layers.DNSTypePTR:
		return strings.TrimSuffix(string(record.PTR), ".")
	case layers.DNSTypeTXT:
		parts := make([]string, 0, len(record.TXTs))
		for _, txt := range record.TXTs {
			parts = append(parts, string(txt))
		}
		return strings.Join(parts, " ")
	default:
		if len(record.Data) > 0 {
			return hex.EncodeToString(record.Data)
		}
		return ""
	}
}

func innerSummary(inner *Inner) string {
	if inner == nil {
		return ""
	}
	if inner.DNS != nil {
		return dnsSummary(inner.DNS)
	}
	if inner.Protocol == "TCP" && inner.TCP != nil {
		parts := []string{fmt.Sprintf("%d -> %d", inner.SrcPort, inner.DstPort)}
		if len(inner.TCP.Flags) > 0 {
			parts = append(parts, fmt.Sprintf("[%s]", strings.Join(inner.TCP.Flags, ", ")))
		}
		if inner.TCP.Seq != nil {
			parts = append(parts, fmt.Sprintf("Seq=%d", *inner.TCP.Seq))
		}
		if inner.TCP.Ack != nil {
			parts = append(parts, fmt.Sprintf("Ack=%d", *inner.TCP.Ack))
		}
		if inner.TCP.Window != nil {
			parts = append(parts, fmt.Sprintf("Win=%d", *inner.TCP.Window))
		}
		if inner.PayloadLength != nil {
			parts = append(parts, fmt.Sprintf("Len=%d", *inner.PayloadLength))
		}
		if inner.TCP.TimestampValue != nil {
			parts = append(parts, fmt.Sprintf("TSval=%d", *inner.TCP.TimestampValue))
		}
		if inner.TCP.TimestampEchoReply != nil {
			parts = append(parts, fmt.Sprintf("TSecr=%d", *inner.TCP.TimestampEchoReply))
		}
		return strings.Join(parts, " ")
	}
	if inner.Protocol == "UDP" {
		if inner.PayloadLength != nil {
			return fmt.Sprintf("%d -> %d Len=%d", inner.SrcPort, inner.DstPort, *inner.PayloadLength)
		}
		return fmt.Sprintf("%d -> %d", inner.SrcPort, inner.DstPort)
	}
	return inner.Protocol
}

func dnsSummary(dns *DNSMeta) string {
	if dns == nil {
		return ""
	}

	kind := "Standard query"
	if dns.Response {
		kind = "Standard query response"
	}

	parts := []string{fmt.Sprintf("%s 0x%04x", kind, dns.ID)}
	if len(dns.Questions) > 0 {
		question := dns.Questions[0]
		parts = append(parts, question.Type, question.Name)
	}
	for _, answer := range dns.Answers {
		if answer.Data == "" {
			continue
		}
		parts = append(parts, answer.Type, answer.Data)
	}
	return strings.Join(parts, " ")
}

func discoSummary(meta *DiscoMeta) string {
	if meta == nil || meta.Frame == nil {
		return ""
	}
	parts := []string{meta.Frame.Type}
	if meta.SrcIP != "" || meta.SrcPort != 0 {
		parts = append(parts, fmt.Sprintf("from %s:%d", meta.SrcIP, meta.SrcPort))
	}
	return strings.Join(parts, " ")
}

func populateDiscoColumns(record *Record) {
	if record == nil || record.DiscoMeta == nil {
		return
	}
	record.Protocol = "TSMP/DISCO"
	record.Src = endpointOrIP(record.DiscoMeta.SrcIP, record.DiscoMeta.SrcPort)
	if record.DiscoMeta.Frame != nil && record.DiscoMeta.Frame.PongSrc != "" {
		record.Dst = endpointOrIP(record.DiscoMeta.Frame.PongSrc, record.DiscoMeta.Frame.PongSrcPort)
	}
	record.Info = discoSummary(record.DiscoMeta)
	record.Summary = record.Info
}

func populateInnerColumns(record *Record) {
	if record == nil || record.Inner == nil {
		return
	}
	record.Src = record.Inner.SrcIP
	record.Dst = record.Inner.DstIP
	record.Protocol = record.Inner.Protocol
	record.PayloadLength = record.Inner.PayloadLength
	record.PayloadPreview = payloadPreview(record.Inner.PayloadHex)
	record.Info = innerSummary(record.Inner)
	record.Summary = record.Info
	if record.Inner.TCP != nil {
		record.RelativeSeq = record.Inner.TCP.RelativeSeq
		record.RelativeAck = record.Inner.TCP.RelativeAck
	}
}

func packetOrigin(pathID uint16) string {
	switch pathID {
	case 2, 3:
		return "synthesized"
	default:
		return "captured"
	}
}

func endpointOrIP(ip string, port uint16) string {
	if ip == "" {
		return ""
	}
	if port == 0 {
		return ip
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func payloadPreview(payloadHex string) string {
	if payloadHex == "" {
		return ""
	}
	bytes, err := hex.DecodeString(payloadHex)
	if err != nil || len(bytes) == 0 {
		return ""
	}
	limit := len(bytes)
	if limit > 32 {
		limit = 32
	}
	preview := make([]byte, 0, limit)
	for _, b := range bytes[:limit] {
		if b < 32 || b > 126 {
			return ""
		}
		preview = append(preview, b)
	}
	return string(preview)
}

func flowEndpoint(ip string, port uint16) string {
	if port == 0 {
		return ip
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func stableFlowID(key string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("flow-%016x", h.Sum64())
}

func canonicalFlowKey(inner *Inner) (key string, direction string, directionalKey string) {
	if inner == nil {
		return "", "", ""
	}

	left := flowEndpoint(inner.SrcIP, inner.SrcPort)
	right := flowEndpoint(inner.DstIP, inner.DstPort)
	directionalKey = left + ">" + right
	ordered := []string{left, right}
	sort.Strings(ordered)
	direction = "forward"
	if ordered[0] != left {
		direction = "reverse"
	}
	key = strings.Join([]string{
		fmt.Sprintf("ip%d", inner.IPVersion),
		inner.Protocol,
		ordered[0],
		ordered[1],
	}, "|")
	return key, direction, directionalKey
}

func intPtr(v int) *int {
	return &v
}

func uint8Ptr(v uint8) *uint8 {
	return &v
}

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}
