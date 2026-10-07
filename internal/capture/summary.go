package capture

import (
	"net/netip"
	"strconv"
	"time"

	"github.com/google/gopacket/layers"
)

const (
	summaryConversationLimit = 100
	summaryEndpointLimit     = 200
	summaryFindingLimit      = 3
	summaryDNSLimit          = 20
	summaryDNSItemLimit      = 4
	summaryTextLimit         = 256
)

var summaryFindingCategories = [...]string{
	"retransmission", "fast_retransmission", "out_of_order", "previous_segment_not_captured",
	"zero_window", "duplicate_ack", "ack_unseen_data", "keepalive", "keepalive_ack",
	"partial_history", "decode_error", "other_analysis",
}

type summaryDocument struct {
	SchemaVersion int                   `json:"schema_version"`
	Mode          string                `json:"mode"`
	Capture       summaryCapture        `json:"capture"`
	Endpoints     []summaryEndpoint     `json:"endpoints"`
	Conversations []summaryConversation `json:"conversations"`
	DNS           summaryDNS            `json:"dns"`
	Findings      []summaryFinding      `json:"findings"`
	Limits        summaryLimits         `json:"limits"`
	Omissions     summaryOmissions      `json:"omissions"`
	Notes         []string              `json:"notes"`
}

type summaryCapture struct {
	PacketCount   uint64            `json:"packet_count"`
	CapturedBytes uint64            `json:"captured_bytes"`
	Start         *time.Time        `json:"start,omitempty"`
	End           *time.Time        `json:"end,omitempty"`
	DurationMS    float64           `json:"duration_ms"`
	DecodeErrors  uint64            `json:"decode_errors"`
	DiscoPackets  uint64            `json:"disco_packets"`
	Protocols     map[string]uint64 `json:"protocols"`
	Paths         map[string]uint64 `json:"paths"`
	Origins       map[string]uint64 `json:"origins"`
}

type summaryLimits struct {
	Conversations   int `json:"conversations"`
	Endpoints       int `json:"endpoints"`
	FindingExamples int `json:"finding_examples_per_category"`
	DNSExamples     int `json:"dns_examples_per_side"`
	DNSQuestions    int `json:"dns_questions_per_example"`
	DNSAnswers      int `json:"dns_answers_per_example"`
	TextCodePoints  int `json:"text_code_points"`
}

type summaryOmissions struct {
	ConversationPackets uint64            `json:"conversation_detail_packets"`
	NoIdentityPackets   uint64            `json:"packets_without_conversation_identity"`
	FindingExamples     map[string]uint64 `json:"finding_examples"`
	DNSQueryExamples    uint64            `json:"dns_query_examples"`
	DNSResponseExamples uint64            `json:"dns_response_examples"`
	DNSQuestions        uint64            `json:"dns_questions"`
	DNSAnswers          uint64            `json:"dns_answers"`
	TextFields          uint64            `json:"text_fields_truncated"`
}

type summaryEndpoint struct {
	ID string `json:"id"`
	IP string `json:"ip"`
}

type summaryConversationEndpoint struct {
	EndpointID string `json:"endpoint_id"`
	Port       uint16 `json:"port"`
}

type summaryTraffic struct {
	Packets       uint64 `json:"packets"`
	CapturedBytes uint64 `json:"captured_bytes"`
}

type summaryConversation struct {
	StreamID   string                      `json:"stream_id,omitempty"`
	A          summaryConversationEndpoint `json:"a"`
	B          summaryConversationEndpoint `json:"b"`
	Protocol   string                      `json:"protocol"`
	IPVersion  int                         `json:"ip_version"`
	FirstFrame int                         `json:"first_frame"`
	LastFrame  int                         `json:"last_frame"`
	StartMS    float64                     `json:"start_ms"`
	EndMS      float64                     `json:"end_ms"`
	AToB       summaryTraffic              `json:"a_to_b"`
	BToA       summaryTraffic              `json:"b_to_a"`
	Paths      map[string]uint64           `json:"paths"`
	Origins    map[string]uint64           `json:"origins"`
	SYNPackets uint64                      `json:"syn_packets"`
	FINPackets uint64                      `json:"fin_packets"`
	RSTPackets uint64                      `json:"rst_packets"`
	start, end time.Time
}

type summaryDNS struct {
	QueryPackets             uint64              `json:"query_packets"`
	ResponsePackets          uint64              `json:"response_packets"`
	MatchedResponsePackets   uint64              `json:"matched_response_packets"`
	UnmatchedResponsePackets uint64              `json:"unmatched_response_packets"`
	ResponseCodes            map[string]uint64   `json:"response_codes"`
	Latency                  summaryLatency      `json:"latency_ms"`
	NegativeLatency          uint64              `json:"negative_latency_observations"`
	Queries                  []summaryDNSExample `json:"query_examples"`
	Responses                []summaryDNSExample `json:"response_examples"`
}

type summaryLatency struct {
	Count uint64   `json:"count"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Mean  *float64 `json:"mean,omitempty"`
}

type summaryDNSAnswer struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
}

type summaryDNSExample struct {
	Frame        int                `json:"frame_number"`
	TimeMS       float64            `json:"time_ms"`
	Questions    []DNSQuestionMeta  `json:"questions"`
	Answers      []summaryDNSAnswer `json:"answers"`
	ResponseCode string             `json:"response_code,omitempty"`
	Status       string             `json:"status,omitempty"`
	PeerFrame    *int               `json:"peer_frame_number,omitempty"`
	LatencyMS    *float64           `json:"latency_ms,omitempty"`
	timestamp    time.Time
}

type summaryFinding struct {
	Category string            `json:"category"`
	Packets  uint64            `json:"packets"`
	Examples []summaryEvidence `json:"examples"`
}

type summaryEvidence struct {
	Frame     int         `json:"frame_number"`
	TimeMS    float64     `json:"time_ms"`
	StreamID  string      `json:"stream_id,omitempty"`
	TCP       *summaryTCP `json:"tcp,omitempty"`
	Error     string      `json:"error,omitempty"`
	timestamp time.Time
}

type summaryTCP struct {
	Seq         *uint32  `json:"seq,omitempty"`
	Ack         *uint32  `json:"ack,omitempty"`
	RelativeSeq *uint32  `json:"relative_seq,omitempty"`
	RelativeAck *uint32  `json:"relative_ack,omitempty"`
	Window      *uint16  `json:"window,omitempty"`
	Flags       []string `json:"flags"`
}

type summaryAccumulator struct {
	doc           summaryDocument
	conversations map[string]int
	endpoints     map[string]string
}

func newSummaryAccumulator() *summaryAccumulator {
	return &summaryAccumulator{conversations: map[string]int{}, endpoints: map[string]string{}, doc: summaryDocument{
		SchemaVersion: 1, Mode: "summary",
		Capture:   summaryCapture{Protocols: map[string]uint64{}, Paths: map[string]uint64{}, Origins: map[string]uint64{}},
		Endpoints: []summaryEndpoint{}, Conversations: []summaryConversation{}, Findings: []summaryFinding{},
		DNS:       summaryDNS{ResponseCodes: map[string]uint64{}, Queries: []summaryDNSExample{}, Responses: []summaryDNSExample{}},
		Limits:    summaryLimits{summaryConversationLimit, summaryEndpointLimit, summaryFindingLimit, summaryDNSLimit, summaryDNSItemLimit, summaryDNSItemLimit, summaryTextLimit},
		Omissions: summaryOmissions{FindingExamples: map[string]uint64{}},
		Notes: []string{
			"Lossy first-seen selection: first 100 canonical conversations, first 20 DNS examples per side, and first 3 examples per finding category; use detailed export or the original PCAP for follow-up.",
			"Conversations merge tuple reuse, not connection lifetimes. A/B directions are canonical, not inferred client/server roles.",
			"Findings are heuristic observations, not proof of root cause or packet loss. Partial capture history can affect analysis.",
			"Packet and captured-byte counts include wrapper bytes and duplicate capture observations, not unique application bytes.",
			"DNS counts describe observed packets and analyzer matches, not unique transactions, unanswered queries, or timeouts.",
			"Metadata remains sensitive, including IP addresses and DNS names; this is not anonymization.",
			"Only summary detail is bounded; existing analyzer DNS correlation state and transient decoded records are outside this bound.",
		},
	}}
}

func summaryProtocol(r Record) string {
	if r.Disco {
		return "DISCO"
	}
	p := r.Protocol
	if r.Inner != nil {
		p = r.Inner.Protocol
	}
	switch p {
	case "TCP", "UDP", "ICMPv4", "ICMPv6":
		return p
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func summaryPath(r Record) string {
	// A short wrapper has no decoded path ID; its zero value is not path 0.
	if r.FrameLength < 2 {
		return "unknown"
	}
	switch r.PathID {
	case 0, 1, 2, 3, discoPathID:
		return strconv.Itoa(int(r.PathID))
	default:
		return "unknown"
	}
}

func summaryOrigin(r Record) string {
	switch r.PacketOrigin {
	case "captured", "synthesized":
		return r.PacketOrigin
	default:
		return "unknown"
	}
}

func (s *summaryAccumulator) add(r Record) {
	c := &s.doc.Capture
	c.PacketCount++
	c.CapturedBytes += uint64(r.FrameLength)
	t := r.Timestamp.UTC()
	if c.Start == nil || t.Before(*c.Start) {
		c.Start = &t
	}
	if c.End == nil || t.After(*c.End) {
		c.End = &t
	}
	c.DurationMS = c.End.Sub(*c.Start).Seconds() * 1000
	if r.Error != "" {
		c.DecodeErrors++
	}
	if r.Disco {
		c.DiscoPackets++
	}
	c.Protocols[summaryProtocol(r)]++
	c.Paths[summaryPath(r)]++
	c.Origins[summaryOrigin(r)]++
	s.addConversation(r)
	s.addDNS(r)
	s.addFindings(r)
}

func (s *summaryAccumulator) addConversation(r Record) {
	i := r.Inner
	if r.Disco || i == nil || (i.IPVersion != 4 && i.IPVersion != 6) || i.Protocol == "" {
		s.doc.Omissions.NoIdentityPackets++
		return
	}
	if _, err := netip.ParseAddr(i.SrcIP); err != nil {
		s.doc.Omissions.NoIdentityPackets++
		return
	}
	if _, err := netip.ParseAddr(i.DstIP); err != nil {
		s.doc.Omissions.NoIdentityPackets++
		return
	}
	key, direction, _ := canonicalFlowKey(i)
	index, exists := s.conversations[key]
	if !exists {
		if len(s.conversations) == summaryConversationLimit {
			s.doc.Omissions.ConversationPackets++
			return
		}
		aIP, bIP, aPort, bPort := i.SrcIP, i.DstIP, i.SrcPort, i.DstPort
		if direction == "reverse" {
			aIP, bIP, aPort, bPort = bIP, aIP, bPort, aPort
		}
		index = len(s.doc.Conversations)
		s.conversations[key] = index
		s.doc.Conversations = append(s.doc.Conversations, summaryConversation{
			StreamID: s.text(r.StreamID), A: summaryConversationEndpoint{s.internEndpoint(aIP), aPort}, B: summaryConversationEndpoint{s.internEndpoint(bIP), bPort},
			Protocol: summaryProtocol(r), IPVersion: i.IPVersion, FirstFrame: r.FrameNumber,
			start: r.Timestamp, end: r.Timestamp, Paths: map[string]uint64{}, Origins: map[string]uint64{},
		})
	}
	c := &s.doc.Conversations[index]
	c.LastFrame = r.FrameNumber
	if r.Timestamp.Before(c.start) {
		c.start = r.Timestamp
	}
	if r.Timestamp.After(c.end) {
		c.end = r.Timestamp
	}
	traffic := &c.AToB
	if direction == "reverse" {
		traffic = &c.BToA
	}
	traffic.Packets++
	traffic.CapturedBytes += uint64(r.FrameLength)
	c.Paths[summaryPath(r)]++
	c.Origins[summaryOrigin(r)]++
	if i.TCP != nil {
		if containsFlag(i.TCP.Flags, "SYN") {
			c.SYNPackets++
		}
		if containsFlag(i.TCP.Flags, "FIN") {
			c.FINPackets++
		}
		if containsFlag(i.TCP.Flags, "RST") {
			c.RSTPackets++
		}
	}
}

func (s *summaryAccumulator) internEndpoint(ip string) string {
	if id, ok := s.endpoints[ip]; ok {
		return id
	}
	id := "e" + strconv.Itoa(len(s.endpoints)+1)
	s.endpoints[ip] = id
	s.doc.Endpoints = append(s.doc.Endpoints, summaryEndpoint{ID: id, IP: ip})
	return id
}

func (s *summaryAccumulator) text(value string) string {
	// Build only the retained prefix, including replacement runes for invalid UTF-8.
	runes := make([]rune, 0, summaryTextLimit)
	for _, r := range value {
		if len(runes) == summaryTextLimit {
			s.doc.Omissions.TextFields++
			break
		}
		runes = append(runes, r)
	}
	return string(runes)
}

func (s *summaryAccumulator) finalize() summaryDocument {
	if s.doc.Capture.Start != nil {
		for index := range s.doc.Conversations {
			c := &s.doc.Conversations[index]
			c.StartMS = c.start.Sub(*s.doc.Capture.Start).Seconds() * 1000
			c.EndMS = c.end.Sub(*s.doc.Capture.Start).Seconds() * 1000
		}
		for _, examples := range [][]summaryDNSExample{s.doc.DNS.Queries, s.doc.DNS.Responses} {
			for index := range examples {
				examples[index].TimeMS = examples[index].timestamp.Sub(*s.doc.Capture.Start).Seconds() * 1000
			}
		}
		for index := range s.doc.Findings {
			for j := range s.doc.Findings[index].Examples {
				e := &s.doc.Findings[index].Examples[j]
				e.TimeMS = e.timestamp.Sub(*s.doc.Capture.Start).Seconds() * 1000
			}
		}
	}
	// Findings are selected independently, but emitted in vocabulary order.
	ordered := make([]summaryFinding, 0, len(s.doc.Findings))
	for _, category := range summaryFindingCategories {
		for _, finding := range s.doc.Findings {
			if finding.Category == category {
				ordered = append(ordered, finding)
				break
			}
		}
	}
	s.doc.Findings = ordered
	return s.doc
}

func (s *summaryAccumulator) addFindings(r Record) {
	var present [len(summaryFindingCategories)]bool
	mark := func(category string) {
		if category == "unseen_segment" {
			category = "previous_segment_not_captured"
		}
		for index, known := range summaryFindingCategories {
			if category == known {
				present[index] = true
				return
			}
		}
		present[len(present)-1] = true
	}
	if r.Error != "" {
		mark("decode_error")
	}
	if a := r.Analysis; a != nil {
		for _, category := range a.Annotations {
			mark(category)
		}
		for index, flag := range []bool{a.Retransmission, a.FastRetransmission, a.OutOfOrder, a.PreviousSegmentNotCaptured, a.ZeroWindow} {
			if flag {
				present[index] = true
			}
		}
		if a.PartialHistory {
			mark("partial_history")
		}
	}
	for index, category := range summaryFindingCategories {
		if !present[index] {
			continue
		}
		var f *summaryFinding
		for j := range s.doc.Findings {
			if s.doc.Findings[j].Category == category {
				f = &s.doc.Findings[j]
				break
			}
		}
		if f == nil {
			s.doc.Findings = append(s.doc.Findings, summaryFinding{Category: category, Examples: []summaryEvidence{}})
			f = &s.doc.Findings[len(s.doc.Findings)-1]
		}
		f.Packets++
		if len(f.Examples) == summaryFindingLimit {
			s.doc.Omissions.FindingExamples[category]++
			continue
		}
		e := summaryEvidence{Frame: r.FrameNumber, timestamp: r.Timestamp, StreamID: s.text(r.StreamID)}
		if category == "decode_error" {
			e.Error = s.text(r.Error)
		}
		if r.Inner != nil && r.Inner.TCP != nil {
			tcp := r.Inner.TCP
			e.TCP = &summaryTCP{Flags: []string{}}
			if tcp.Seq != nil {
				e.TCP.Seq = uint32Ptr(*tcp.Seq)
			}
			if tcp.Ack != nil {
				e.TCP.Ack = uint32Ptr(*tcp.Ack)
			}
			if tcp.RelativeSeq != nil {
				e.TCP.RelativeSeq = uint32Ptr(*tcp.RelativeSeq)
			}
			if tcp.RelativeAck != nil {
				e.TCP.RelativeAck = uint32Ptr(*tcp.RelativeAck)
			}
			if tcp.Window != nil {
				e.TCP.Window = uint16Ptr(*tcp.Window)
			}
			for _, flag := range []string{"FIN", "SYN", "RST", "PSH", "ACK", "URG", "ECE", "CWR", "NS"} {
				if containsFlag(tcp.Flags, flag) {
					e.TCP.Flags = append(e.TCP.Flags, flag)
				}
			}
		}
		f.Examples = append(f.Examples, e)
	}
}

func summaryResponseCode(code string) string {
	if code == "" {
		return "unknown"
	}
	for _, value := range []layers.DNSResponseCode{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 16, 17, 18, 19, 20, 21, 22, 23} {
		if code == value.String() {
			return code
		}
	}
	return "other"
}

func (s *summaryAccumulator) addDNS(r Record) {
	if r.Inner == nil || r.Inner.DNS == nil {
		return
	}
	d := r.Inner.DNS
	stats := &s.doc.DNS
	examples, omitted := &stats.Queries, &s.doc.Omissions.DNSQueryExamples
	status := "query"
	if d.Response {
		stats.ResponsePackets++
		stats.ResponseCodes[summaryResponseCode(d.ResponseCode)]++
		examples, omitted = &stats.Responses, &s.doc.Omissions.DNSResponseExamples
		status = "unmatched_response"
		if d.Status == "matched" {
			status = "matched"
			stats.MatchedResponsePackets++
			if d.ResponseTimeMillis != nil {
				value := *d.ResponseTimeMillis
				if value < 0 {
					stats.NegativeLatency++
				} else {
					l := &stats.Latency
					l.Count++
					if l.Min == nil || value < *l.Min {
						l.Min = float64Ptr(value)
					}
					if l.Max == nil || value > *l.Max {
						l.Max = float64Ptr(value)
					}
					if l.Mean == nil {
						l.Mean = float64Ptr(value)
					} else {
						*l.Mean += (value - *l.Mean) / float64(l.Count)
					}
				}
			}
		} else {
			stats.UnmatchedResponsePackets++
		}
	} else {
		stats.QueryPackets++
	}
	if len(*examples) == summaryDNSLimit {
		(*omitted)++
		return
	}
	e := summaryDNSExample{Frame: r.FrameNumber, timestamp: r.Timestamp, Status: status, Questions: []DNSQuestionMeta{}, Answers: []summaryDNSAnswer{}}
	if d.Response {
		e.ResponseCode = summaryResponseCode(d.ResponseCode)
	}
	if d.PeerFrameNumber != nil {
		e.PeerFrame = intPtr(*d.PeerFrameNumber)
	}
	if d.ResponseTimeMillis != nil {
		e.LatencyMS = float64Ptr(*d.ResponseTimeMillis)
	}
	for index, q := range d.Questions {
		if index == summaryDNSItemLimit {
			s.doc.Omissions.DNSQuestions += uint64(len(d.Questions) - index)
			break
		}
		e.Questions = append(e.Questions, DNSQuestionMeta{Name: s.text(q.Name), Type: s.text(q.Type), Class: s.text(q.Class)})
	}
	for index, a := range d.Answers {
		if index == summaryDNSItemLimit {
			s.doc.Omissions.DNSAnswers += uint64(len(d.Answers) - index)
			break
		}
		e.Answers = append(e.Answers, summaryDNSAnswer{Name: s.text(a.Name), Type: s.text(a.Type), Data: s.text(a.Data)})
	}
	*examples = append(*examples, e)
}
