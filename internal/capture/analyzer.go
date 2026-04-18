package capture

import (
	"fmt"
	"strings"
	"time"
)

const maxTrackedFlows = 256

type Analyzer struct {
	flows        map[string]*flowState
	transactions map[string]*dnsTransactionState
	order        []string
}

type flowState struct {
	id              string
	key             string
	lastFrame       int
	startTime       time.Time
	forward         *directionState
	reverse         *directionState
	requestFrameNum map[string]int
}

type directionState struct {
	key          string
	role         string
	baseSeq      *uint32
	nextExpected uint32
	lastAck      *uint32
	seen         bool
	lastFrame    int
	lastTime     time.Time
	packetCount  int
}

type dnsTransactionState struct {
	id           string
	requestFrame int
	requestTime  time.Time
	status       string
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		flows:        make(map[string]*flowState),
		transactions: make(map[string]*dnsTransactionState),
	}
}

func (a *Analyzer) Analyze(record Record) Record {
	if record.Disco || record.Inner == nil {
		return record
	}

	flowKey, direction, directionalKey := canonicalFlowKey(record.Inner)
	if flowKey == "" {
		return record
	}

	flow := a.ensureFlow(flowKey, directionalKey, record.FrameNumber)
	record.FlowID = flow.id
	record.StreamID = flow.id
	record.ConversationKey = flow.key
	record.FlowDir = direction
	record.Analysis = ensureAnalysis(record.Analysis)

	current, peer := flow.directionFor(directionalKey)
	assignTransportRoles(flow, current, peer)
	current.packetCount++
	record.StreamPacketNumber = intPtr(current.packetCount)
	record.TransportDirection = current.role
	if !flow.startTime.IsZero() {
		delta := record.Timestamp.Sub(flow.startTime).Seconds()
		record.TimeSinceStreamStart = float64Ptr(delta)
	}
	if !current.lastTime.IsZero() {
		delta := record.Timestamp.Sub(current.lastTime).Seconds()
		record.TimeSincePreviousInStream = float64Ptr(delta)
	}
	record.Analysis.PartialHistory = !current.seen || !peer.seen

	if record.Inner.Protocol == "TCP" && record.Inner.TCP != nil {
		a.analyzeTCP(&record, current, peer)
	}

	if record.Inner.DNS != nil {
		a.analyzeDNS(&record, flow)
	}

	flow.lastFrame = record.FrameNumber
	if flow.startTime.IsZero() {
		flow.startTime = record.Timestamp
	}
	current.lastTime = record.Timestamp
	a.touch(flowKey)
	a.evict()
	record.Summary = analyzedSummary(record)
	record.Info = record.Summary
	return record
}

func ensureAnalysis(analysis *Analysis) *Analysis {
	if analysis != nil {
		return analysis
	}
	return &Analysis{}
}

func (a *Analyzer) ensureFlow(key, directionalKey string, frame int) *flowState {
	if flow, ok := a.flows[key]; ok {
		return flow
	}

	flow := &flowState{
		id:              stableFlowID(key),
		key:             key,
		lastFrame:       frame,
		startTime:       time.Time{},
		forward:         &directionState{key: directionalKey},
		reverse:         &directionState{},
		requestFrameNum: make(map[string]int),
	}
	a.flows[key] = flow
	a.order = append(a.order, key)
	return flow
}

func (f *flowState) directionFor(directionalKey string) (current, peer *directionState) {
	if f.forward.key == "" || f.forward.key == directionalKey {
		f.forward.key = directionalKey
		return f.forward, f.reverse
	}
	if f.reverse.key == "" {
		f.reverse.key = directionalKey
	}
	if f.reverse.key == directionalKey {
		return f.reverse, f.forward
	}
	return f.forward, f.reverse
}

func assignTransportRoles(flow *flowState, current, peer *directionState) {
	if flow == nil || current == nil || peer == nil {
		return
	}
	if current.role == "" && peer.role == "" {
		current.role = "client_to_server"
		peer.role = "server_to_client"
		return
	}
	if current.role == "" {
		if peer.role == "client_to_server" {
			current.role = "server_to_client"
		} else {
			current.role = "client_to_server"
		}
	}
}

func (a *Analyzer) touch(key string) {
	for i, existing := range a.order {
		if existing != key {
			continue
		}
		a.order = append(append(a.order[:i:i], a.order[i+1:]...), key)
		return
	}
	a.order = append(a.order, key)
}

func (a *Analyzer) evict() {
	for len(a.order) > maxTrackedFlows {
		oldest := a.order[0]
		a.order = a.order[1:]
		delete(a.flows, oldest)
	}
}

func (a *Analyzer) analyzeTCP(record *Record, current, peer *directionState) {
	tcp := record.Inner.TCP
	seq := *tcp.Seq
	ack := *tcp.Ack

	if !current.seen {
		current.baseSeq = uint32Ptr(seq)
		current.nextExpected = seq + tcpSegmentLength(tcp, record.Inner.PayloadLength)
		current.seen = true
		current.lastFrame = record.FrameNumber
	} else {
		tcp.RelativeSeq = uint32Ptr(seq - *current.baseSeq)
		segmentLen := tcpSegmentLength(tcp, record.Inner.PayloadLength)
		if segmentLen > 0 {
			switch {
			case seq < current.nextExpected:
				if containsString(record.Analysis.Annotations, "duplicate_ack") {
					appendUnique(&record.Analysis.Annotations, "fast_retransmission")
					record.Analysis.FastRetransmission = true
				}
				appendUnique(&record.Analysis.Annotations, "retransmission")
				record.Analysis.Retransmission = true
			case seq > current.nextExpected:
				appendUnique(&record.Analysis.Annotations, "out_of_order")
				appendUnique(&record.Analysis.Annotations, "unseen_segment")
				appendUnique(&record.Analysis.Annotations, "previous_segment_not_captured")
				record.Analysis.OutOfOrder = true
				record.Analysis.PreviousSegmentNotCaptured = true
			}
		}
		if seq+segmentLen > current.nextExpected {
			current.nextExpected = seq + segmentLen
		}
		current.lastFrame = record.FrameNumber
	}

	if current.baseSeq != nil && tcp.RelativeSeq == nil {
		tcp.RelativeSeq = uint32Ptr(seq - *current.baseSeq)
	}

	if peer.baseSeq != nil {
		tcp.RelativeAck = uint32Ptr(ack - *peer.baseSeq)
	}
	if current.lastAck != nil && ack == *current.lastAck {
		appendUnique(&record.Analysis.Annotations, "duplicate_ack")
	}
	current.lastAck = uint32Ptr(ack)
	if tcp.Window != nil && *tcp.Window == 0 {
		appendUnique(&record.Analysis.Annotations, "zero_window")
		record.Analysis.ZeroWindow = true
	}
	if record.Inner.PayloadLength != nil && *record.Inner.PayloadLength == 0 && seq+1 == current.nextExpected && ack > 0 {
		appendUnique(&record.Analysis.Annotations, "keepalive")
	}
	if record.Inner.PayloadLength != nil && *record.Inner.PayloadLength == 0 && current.lastAck != nil && ack == *current.lastAck {
		appendUnique(&record.Analysis.Annotations, "keepalive_ack")
	}
	if peer.baseSeq == nil && ack > 0 {
		appendUnique(&record.Analysis.Annotations, "ack_unseen_data")
	}
	if len(record.Analysis.Annotations) > 0 {
		record.Analysis.Notes = append(record.Analysis.Notes, strings.Join(record.Analysis.Annotations, ", "))
	}
	record.RelativeSeq = tcp.RelativeSeq
	record.RelativeAck = tcp.RelativeAck
}

func tcpSegmentLength(tcp *TCPMeta, payloadLength *int) uint32 {
	if tcp == nil {
		return 0
	}
	length := uint32(0)
	if payloadLength != nil {
		length = uint32(*payloadLength)
	}
	if containsFlag(tcp.Flags, "SYN") || containsFlag(tcp.Flags, "FIN") {
		length++
	}
	return length
}

func containsFlag(flags []string, target string) bool {
	for _, flag := range flags {
		if flag == target {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendUnique(values *[]string, value string) {
	for _, existing := range *values {
		if existing == value {
			return
		}
	}
	*values = append(*values, value)
}

func (a *Analyzer) analyzeDNS(record *Record, flow *flowState) {
	dns := record.Inner.DNS
	key := dnsTransactionKey(record.FlowID, dns)
	if key == "" {
		return
	}

	if dns.Response {
		if tx, ok := a.transactions[key]; ok {
			dns.TransactionID = tx.id
			dns.PeerFrameNumber = intPtr(tx.requestFrame)
			status := "matched"
			dns.Status = status
			delta := record.Timestamp.Sub(tx.requestTime).Seconds() * 1000
			dns.ResponseTimeMillis = float64Ptr(delta)
			appendUnique(&record.Analysis.Annotations, "dns_response_matched")
		} else {
			dns.TransactionID = stableFlowID(key)
			dns.Status = "unmatched_response"
			appendUnique(&record.Analysis.Annotations, "dns_response_unmatched")
		}
		return
	}

	txID := stableFlowID(key)
	dns.TransactionID = txID
	dns.Status = "query"
	a.transactions[key] = &dnsTransactionState{
		id:           txID,
		requestFrame: record.FrameNumber,
		requestTime:  record.Timestamp,
		status:       "query",
	}
	flow.requestFrameNum[key] = record.FrameNumber
}

func dnsTransactionKey(flowID string, dns *DNSMeta) string {
	if dns == nil || flowID == "" {
		return ""
	}
	parts := []string{flowID, fmt.Sprintf("%d", dns.ID), dns.OpCode}
	for _, question := range dns.Questions {
		parts = append(parts, question.Name, question.Type)
	}
	return strings.Join(parts, "|")
}

func analyzedSummary(record Record) string {
	parts := make([]string, 0, 2)
	if record.Summary != "" {
		parts = append(parts, record.Summary)
	}
	if record.Analysis != nil && len(record.Analysis.Annotations) > 0 {
		parts = append(parts, fmt.Sprintf("Annotations=%s", strings.Join(record.Analysis.Annotations, ",")))
	}
	return strings.Join(parts, " ")
}

func float64Ptr(v float64) *float64 {
	return &v
}
