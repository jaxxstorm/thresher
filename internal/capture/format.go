package capture

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OutputFormat string

const (
	FormatJSONL        OutputFormat = "jsonl"
	FormatJSONLCompact OutputFormat = "jsonl-compact"
	FormatSummary      OutputFormat = "summary"
	FormatPacketList   OutputFormat = "packet-list"
)

func ParseOutputFormat(v string) (OutputFormat, error) {
	format := OutputFormat(v)
	switch format {
	case "", FormatJSONL:
		return FormatJSONL, nil
	case FormatJSONLCompact:
		return FormatJSONLCompact, nil
	case FormatSummary:
		return FormatSummary, nil
	case FormatPacketList:
		return FormatPacketList, nil
	default:
		return "", fmt.Errorf("unsupported format %q", v)
	}
}

type RecordWriter interface {
	WriteRecord(Record) error
}

func NewRecordWriter(w io.Writer, format OutputFormat) (RecordWriter, error) {
	switch format {
	case FormatJSONL:
		return &jsonlWriter{enc: NewEncoder(w)}, nil
	case FormatJSONLCompact:
		return &compactWriter{enc: json.NewEncoder(w)}, nil
	case FormatSummary:
		return &summaryWriter{w: w}, nil
	case FormatPacketList:
		return &packetListWriter{w: w}, nil
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

type jsonlWriter struct{ enc *Encoder }

func (w *jsonlWriter) WriteRecord(record Record) error { return w.enc.Encode(record) }

type compactWriter struct{ enc *json.Encoder }

func (w *compactWriter) WriteRecord(record Record) error {
	if w.enc == nil {
		return fmt.Errorf("compact encoder not initialized")
	}
	w.enc.SetEscapeHTML(false)
	return w.enc.Encode(compactRecord(record))
}

type summaryWriter struct{ w io.Writer }

func (w *summaryWriter) WriteRecord(record Record) error {
	_, err := fmt.Fprintln(w.w, summaryLine(record))
	return err
}

type packetListWriter struct{ w io.Writer }

func (w *packetListWriter) WriteRecord(record Record) error {
	_, err := fmt.Fprintln(w.w, packetListLine(record))
	return err
}

type compactView struct {
	Number               int        `json:"number"`
	Time                 string     `json:"time,omitempty"`
	Src                  string     `json:"src,omitempty"`
	Dst                  string     `json:"dst,omitempty"`
	Protocol             string     `json:"protocol,omitempty"`
	Length               int        `json:"length"`
	Info                 string     `json:"info,omitempty"`
	Path                 string     `json:"path,omitempty"`
	PathID               uint16     `json:"path_id"`
	Disco                bool       `json:"disco"`
	StreamID             string     `json:"stream_id,omitempty"`
	ConversationKey      string     `json:"conversation_key,omitempty"`
	StreamPacketNumber   *int       `json:"stream_packet_number,omitempty"`
	TimeSinceStreamStart *float64   `json:"time_since_stream_start,omitempty"`
	TimeSincePrevious    *float64   `json:"time_since_previous_in_stream,omitempty"`
	TransportDirection   string     `json:"transport_direction,omitempty"`
	PayloadLength        *int       `json:"payload_length,omitempty"`
	PayloadPreview       string     `json:"payload_preview,omitempty"`
	Analysis             *Analysis  `json:"analysis,omitempty"`
	Inner                *Inner     `json:"inner,omitempty"`
	DiscoMeta            *DiscoMeta `json:"disco_meta,omitempty"`
	Error                string     `json:"error,omitempty"`
}

func compactRecord(record Record) compactView {
	return compactView{
		Number:               record.Number,
		Time:                 record.Time,
		Src:                  record.Src,
		Dst:                  record.Dst,
		Protocol:             record.Protocol,
		Length:               record.Length,
		Info:                 record.Info,
		Path:                 record.Path,
		PathID:               record.PathID,
		Disco:                record.Disco,
		StreamID:             record.StreamID,
		ConversationKey:      record.ConversationKey,
		StreamPacketNumber:   record.StreamPacketNumber,
		TimeSinceStreamStart: record.TimeSinceStreamStart,
		TimeSincePrevious:    record.TimeSincePreviousInStream,
		TransportDirection:   record.TransportDirection,
		PayloadLength:        record.PayloadLength,
		PayloadPreview:       record.PayloadPreview,
		Analysis:             record.Analysis,
		Inner:                record.Inner,
		DiscoMeta:            record.DiscoMeta,
		Error:                record.Error,
	}
}

func summaryLine(record Record) string {
	parts := []string{
		fmt.Sprintf("%d", record.Number),
		record.Time,
		record.Src,
		record.Dst,
		record.Protocol,
		fmt.Sprintf("%d", record.Length),
		record.Info,
	}
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	return strings.Join(trimmed, " ")
}

func packetListLine(record Record) string {
	parts := []string{
		fmt.Sprintf("%d", record.Number),
		record.Time,
		record.Src,
		record.Dst,
		record.Protocol,
		fmt.Sprintf("%d", record.Length),
		record.Info,
	}
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	return strings.Join(trimmed, "\t")
}
