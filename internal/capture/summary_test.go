package capture

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSummaryCapture(t *testing.T) {
	s := newSummaryAccumulator()
	if s.doc.Capture.PacketCount != 0 || s.doc.Capture.CapturedBytes != 0 || s.doc.Capture.DurationMS != 0 || s.doc.Capture.Start != nil || s.doc.Capture.End != nil {
		t.Fatalf("nonempty capture: %+v", s.doc.Capture)
	}
	packets := [][]byte{
		wrapPacket(0, nil, nil, mustIPv4TCPPacket(t)),
		wrapPacket(1, nil, nil, mustIPv6UDPPacket(t)),
		wrapPacket(2, nil, nil, mustIPv4TCPPacket(t)),
		wrapPacket(3, nil, nil, mustIPv6UDPPacket(t)),
		wrapPacket(254, nil, nil, discoMetadata(1)),
		wrapPacket(256, nil, nil, mustIPv4TCPPacket(t)),
		{1}, {1, 0, 99},
	}
	var total uint64
	base := time.Unix(100, 0)
	for i, p := range packets {
		r, err := DecodePacket(i+1, base.Add(-time.Duration(i)*125*time.Microsecond), p)
		if err != nil {
			r = RecordDecodeErrorFrom(r, err)
		}
		s.add(r)
		total += uint64(len(p))
	}
	c := s.doc.Capture
	if c.PacketCount != 8 || c.CapturedBytes != total || c.DecodeErrors != 2 || c.DiscoPackets != 1 || c.DurationMS != 0.875 || !c.End.Equal(base) || !c.Start.Equal(base.Add(-875*time.Microsecond)) {
		t.Fatalf("incorrect totals: %+v, bytes want %d", c, total)
	}
	for name, pair := range map[string][2]map[string]uint64{
		"protocols": {c.Protocols, {"TCP": 3, "UDP": 2, "DISCO": 1, "unknown": 2}},
		"paths":     {c.Paths, {"0": 1, "1": 2, "2": 1, "3": 1, "254": 1, "unknown": 2}},
		"origins":   {c.Origins, {"captured": 5, "synthesized": 2, "unknown": 1}},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s: got %v want %v", name, pair[0], pair[1])
		}
	}
}

func summaryTestRecord(frame int) Record {
	return Record{FrameNumber: frame, Timestamp: time.Unix(100, int64(frame)*125000), FrameLength: 64, PathID: 1, PacketOrigin: "captured",
		Inner: &Inner{IPVersion: 4, Protocol: "TCP", SrcIP: "100.64.0.1", DstIP: "100.64.0.2", SrcPort: 1000, DstPort: 443,
			TCP: &TCPMeta{Seq: uint32Ptr(1), Ack: uint32Ptr(0), Window: uint16Ptr(100), Flags: []string{"SYN", "SYN", "FIN", "RST"}}}}
}

func TestSummaryConversations(t *testing.T) {
	for _, version := range []int{4, 6} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			s := newSummaryAccumulator()
			a := NewAnalyzer()
			r := summaryTestRecord(1)
			r.Inner.IPVersion = version
			if version == 6 {
				r.Inner.SrcIP, r.Inner.DstIP = "fd7a::1", "fd7a::2"
			}
			s.add(a.Analyze(r))
			r.Inner.SrcIP, r.Inner.DstIP = r.Inner.DstIP, r.Inner.SrcIP
			r.Inner.SrcPort, r.Inner.DstPort = r.Inner.DstPort, r.Inner.SrcPort
			r.FrameNumber, r.Timestamp = 2, time.Unix(100, 0)
			s.add(a.Analyze(r))
			// Reusing the tuple and losing analyzer history must not reset summary counts.
			for n := 0; n <= maxTrackedFlows; n++ {
				x := summaryTestRecord(n + 3)
				x.Inner.SrcPort = uint16(2000 + n)
				a.Analyze(x)
			}
			key, _, _ := canonicalFlowKey(r.Inner)
			if _, ok := a.flows[key]; ok {
				t.Fatal("test did not evict original flow")
			}
			r.FrameNumber, r.Timestamp = 300, time.Unix(101, 0)
			s.add(a.Analyze(r))
			d := s.finalize()
			if len(d.Conversations) != 1 || len(d.Endpoints) != 2 {
				t.Fatalf("unexpected detail: %+v", d)
			}
			c := d.Conversations[0]
			if c.StreamID != stableFlowID(key) || c.A.EndpointID != "e1" || c.B.EndpointID != "e2" || c.A.Port != 1000 || c.B.Port != 443 || c.AToB.Packets != 1 || c.BToA.Packets != 2 || c.AToB.CapturedBytes != 64 || c.BToA.CapturedBytes != 128 || c.FirstFrame != 1 || c.LastFrame != 300 || c.StartMS != 0 || c.EndMS != 1000 || c.SYNPackets != 3 || c.FINPackets != 3 || c.RSTPackets != 3 || c.Paths["1"] != 3 || c.Origins["captured"] != 3 {
				t.Fatalf("incorrect conversation: %+v", c)
			}
		})
	}
}

func TestSummaryConversationIdentityAndCapacity(t *testing.T) {
	s := newSummaryAccumulator()
	for n := 0; n < 101; n++ {
		r := summaryTestRecord(n + 1)
		r.Inner.SrcIP, r.Inner.DstIP = fmt.Sprintf("10.0.%d.1", n), fmt.Sprintf("10.0.%d.2", n)
		r.StreamID = "same-hash"
		s.add(r)
	}
	if len(s.conversations) != 100 || len(s.endpoints) != 200 || len(s.doc.Endpoints) != 200 || len(s.doc.Conversations) != 100 || s.doc.Omissions.ConversationPackets != 1 {
		t.Fatalf("caps: %+v", s.doc.Omissions)
	}
	r := summaryTestRecord(102)
	r.Inner.SrcIP, r.Inner.DstIP = "10.0.0.2", "10.0.0.1"
	r.Inner.SrcPort, r.Inner.DstPort = 443, 1000
	s.add(r)
	s.add(Record{FrameLength: 1})
	s.add(Record{Disco: true, Inner: r.Inner})
	s.add(Record{Inner: &Inner{IPVersion: 4, Protocol: "TCP"}})
	if s.doc.Conversations[0].BToA.Packets != 1 || s.doc.Omissions.ConversationPackets != 1 || s.doc.Omissions.NoIdentityPackets != 3 || s.doc.Capture.PacketCount != 105 {
		t.Fatalf("overflow/identity: %+v", s.doc)
	}

	s = newSummaryAccumulator()
	for _, version := range []int{4, 6} {
		for _, protocol := range []string{"TCP", "UDP"} {
			r := summaryTestRecord(1)
			r.Inner.IPVersion = version
			r.Inner.Protocol = protocol
			r.StreamID = "same-hash"
			s.add(r)
		}
	}
	if len(s.conversations) != 4 || len(s.endpoints) != 2 {
		t.Fatalf("full key identity lost: %+v", s.doc)
	}
}

func TestSummaryFiniteCaptureBuckets(t *testing.T) {
	s := newSummaryAccumulator()
	for i, p := range []string{"TCP", "UDP", "ICMPv4", "ICMPv6", "", "arbitrary", "another"} {
		s.add(Record{FrameNumber: i + 1, FrameLength: 4, Protocol: p, PathID: 65535, PacketOrigin: p})
	}
	if len(s.doc.Capture.Protocols) != 6 || s.doc.Capture.Protocols["other"] != 2 || !reflect.DeepEqual(s.doc.Capture.Paths, map[string]uint64{"unknown": 7}) || !reflect.DeepEqual(s.doc.Capture.Origins, map[string]uint64{"unknown": 7}) {
		t.Fatalf("unbounded buckets: %+v", s.doc.Capture)
	}
	if _, err := json.Marshal(s.doc); err != nil {
		t.Fatal(err)
	}
}

func TestSummaryDNSObservations(t *testing.T) {
	s := newSummaryAccumulator()
	a := NewAnalyzer()
	r := summaryTestRecord(1)
	r.Inner.Protocol, r.Inner.TCP = "UDP", nil
	r.Inner.DNS = &DNSMeta{ID: 1, Questions: []DNSQuestionMeta{{Name: "example.com", Type: "A", Class: "IN"}}}
	s.add(a.Analyze(r))
	for n, delta := range []time.Duration{time.Millisecond, 3 * time.Millisecond, -time.Millisecond} {
		x := r
		inner := *r.Inner
		dns := *inner.DNS
		x.Inner = &inner
		inner.DNS = &dns
		x.FrameNumber = n + 2
		x.Timestamp = r.Timestamp.Add(delta)
		dns.Response = true
		dns.ResponseCode = "No Error"
		s.add(a.Analyze(x))
	}
	x := summaryTestRecord(5)
	x.Inner.DNS = &DNSMeta{Response: true, Status: "matched", PeerFrameNumber: intPtr(1), ResponseCode: "Non-Existent Domain"}
	s.add(x) // Reported match with no reported latency.
	x.FrameNumber = 6
	x.Inner.DNS = &DNSMeta{Response: true, Status: "unmatched_response", ResponseCode: "arbitrary", ResponseTimeMillis: float64Ptr(999)}
	s.add(x)
	d := s.finalize().DNS
	if d.QueryPackets != 1 || d.ResponsePackets != 5 || d.MatchedResponsePackets != 4 || d.UnmatchedResponsePackets != 1 || d.NegativeLatency != 1 || d.Latency.Count != 2 || *d.Latency.Min != 1 || *d.Latency.Max != 3 || *d.Latency.Mean != 2 || d.ResponseCodes["No Error"] != 3 || d.ResponseCodes["other"] != 1 {
		t.Fatalf("DNS counts: %+v", d)
	}
	for index := 0; index < 4; index++ {
		if d.Responses[index].PeerFrame == nil || *d.Responses[index].PeerFrame != 1 {
			t.Fatalf("missing supplied peer: %+v", d.Responses[index])
		}
	}
	if d.Responses[2].LatencyMS == nil || *d.Responses[2].LatencyMS != -1 || d.Responses[3].LatencyMS != nil || d.Queries[0].TimeMS != 1 {
		t.Fatalf("timing evidence: %+v", d)
	}
}

func TestSummaryDNSLimits(t *testing.T) {
	s := newSummaryAccumulator()
	for _, response := range []bool{false, true} {
		for n := 1; n <= 23; n++ {
			r := summaryTestRecord(n)
			r.Inner.DNS = &DNSMeta{Response: response, Questions: make([]DNSQuestionMeta, 6), Answers: make([]DNSRecordMeta, 7)}
			if response {
				r.Inner.DNS.Status = "matched"
				r.Inner.DNS.ResponseTimeMillis = float64Ptr(float64(n))
			}
			s.add(r)
		}
	}
	d := s.doc.DNS
	o := s.doc.Omissions
	if d.QueryPackets != 23 || d.ResponsePackets != 23 || d.MatchedResponsePackets != 23 || d.Latency.Count != 23 || *d.Latency.Min != 1 || *d.Latency.Max != 23 || *d.Latency.Mean != 12 || len(d.Queries) != 20 || len(d.Responses) != 20 || o.DNSQueryExamples != 3 || o.DNSResponseExamples != 3 || o.DNSQuestions != 80 || o.DNSAnswers != 120 || d.ResponseCodes["unknown"] != 23 {
		t.Fatalf("limits: %+v, %+v", d, o)
	}
	for _, side := range [][]summaryDNSExample{d.Queries, d.Responses} {
		for index, e := range side {
			if e.Frame != index+1 || len(e.Questions) != 4 || len(e.Answers) != 4 {
				t.Fatalf("wrong retained example: %+v", e)
			}
		}
	}
}

func TestSummaryFindings(t *testing.T) {
	s := newSummaryAccumulator()
	for n := 1; n <= 100; n++ {
		r := summaryTestRecord(n)
		r.Inner.SrcPort = uint16(n)
		s.add(r)
	}
	for n := 101; n <= 105; n++ {
		r := summaryTestRecord(n)
		r.StreamID = "outside-retention"
		r.Analysis = &Analysis{
			Retransmission: true, FastRetransmission: true, OutOfOrder: true, PreviousSegmentNotCaptured: true, ZeroWindow: true, PartialHistory: true,
			Annotations: append(append([]string{}, summaryFindingCategories[:]...), "unseen_segment", "unbounded-category-1", "unbounded-category-2"),
		}
		r.Error = "decode failure"
		s.add(r)
	}
	d := s.finalize()
	if len(d.Findings) != len(summaryFindingCategories) || len(d.Omissions.FindingExamples) != len(summaryFindingCategories) || d.Omissions.ConversationPackets != 5 || len(d.Endpoints) != 2 {
		t.Fatalf("finding independence: %+v", d.Omissions)
	}
	for index, f := range d.Findings {
		if f.Category != summaryFindingCategories[index] || f.Packets != 5 || len(f.Examples) != 3 || d.Omissions.FindingExamples[f.Category] != 2 {
			t.Fatalf("finding dedup/overflow: %+v", f)
		}
		for j, e := range f.Examples {
			if e.Frame != j+101 || e.StreamID != "outside-retention" || e.TCP == nil || *e.TCP.Seq != 1 || *e.TCP.Window != 100 || len(e.TCP.Flags) != 3 || e.TimeMS != float64(j+100)*0.125 {
				t.Fatalf("finding evidence: %+v", e)
			}
		}
	}
}

func TestSummaryTextAndExcludedFields(t *testing.T) {
	s := newSummaryAccumulator()
	long := strings.Repeat("\u754c", 257)
	r := summaryTestRecord(1)
	r.RawHex, r.PayloadPreview, r.Info, r.Summary = "RAW_SECRET", "PREVIEW_SECRET", "INFO_SECRET", "SUMMARY_SECRET"
	r.Inner.RawHex, r.Inner.PayloadHex = "INNER_SECRET", "PAYLOAD_SECRET"
	r.Inner.TCP.Checksum = uint16Ptr(123)
	r.Inner.TCP.Options = []TCPOptionMeta{{Type: "OPTION_SECRET", Data: "OPTION_DATA_SECRET"}}
	r.Inner.TCP.Flags = append(r.Inner.TCP.Flags, "FLAG_SECRET")
	r.Inner.DNS = &DNSMeta{RawHex: "DNS_RAW_SECRET", Questions: []DNSQuestionMeta{{Name: long, Type: long, Class: long}}, Answers: []DNSRecordMeta{{Name: long, Type: long, Data: long}}, Authorities: []DNSRecordMeta{{Name: "AUTHORITY_SECRET"}}, Additionals: []DNSRecordMeta{{Name: "ADDITIONAL_SECRET"}}}
	r.Error = long
	r.Analysis = &Analysis{Annotations: []string{"CUSTOM_ANNOTATION_SECRET"}, Notes: []string{"NOTE_SECRET"}}
	s.add(r)
	// Only retained fields count as truncated; discarded DNS/finding examples do not.
	for n := 2; n <= 25; n++ {
		r.FrameNumber = n
		s.add(r)
	}
	d := s.finalize()
	if d.Omissions.TextFields != 20*6+3 || d.Omissions.DNSQueryExamples != 5 || d.Omissions.FindingExamples["decode_error"] != 22 {
		t.Fatalf("text accounting: %+v", d.Omissions)
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	assertExportNoRaw(t, data)
	for _, forbidden := range []string{"SECRET", `"checksum"`, `"options"`, `"authorities"`, `"additionals"`, `"info"`, `"summary":`, `"packets":[`} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("excluded content %s in output", forbidden)
		}
	}
	for _, value := range []string{d.DNS.Queries[0].Questions[0].Name, d.DNS.Queries[0].Questions[0].Type, d.DNS.Queries[0].Questions[0].Class, d.DNS.Queries[0].Answers[0].Name, d.DNS.Queries[0].Answers[0].Type, d.DNS.Queries[0].Answers[0].Data, d.Findings[0].Examples[0].Error} {
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) != 256 {
			t.Fatalf("invalid truncation: %q", value)
		}
	}
	before := s.doc.Omissions.TextFields
	if got := s.text(strings.Repeat("a", 256)); len(got) != 256 || s.doc.Omissions.TextFields != before {
		t.Fatal("exact-limit text counted as truncated")
	}
	if got := s.text("bad\xffutf8"); !utf8.ValidString(got) {
		t.Fatal("invalid UTF-8 retained")
	}
}

func TestSummaryFindingCategoryOrderAndTextCopies(t *testing.T) {
	s := newSummaryAccumulator()
	r := summaryTestRecord(1)
	r.StreamID = strings.Repeat("s", 257)
	r.Error = "error"
	s.add(r)
	r.Error = ""
	r.Analysis = &Analysis{Retransmission: true}
	s.add(r)
	*r.Inner.TCP.Seq = 999
	d := s.finalize()
	if d.Findings[0].Category != "retransmission" || d.Findings[1].Category != "decode_error" || *d.Findings[0].Examples[0].TCP.Seq != 1 || len(d.Conversations[0].StreamID) != 256 || d.Omissions.TextFields != 3 {
		t.Fatalf("order/copy/text limits: %+v", d)
	}
}
