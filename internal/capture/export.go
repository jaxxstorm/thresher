package capture

import (
	"context"
	"fmt"
	"io"
)

// ExportJSON streams a versioned JSON document. On failure, output may be partial.
func ExportJSON(ctx context.Context, output io.Writer, open StreamOpener, includeRaw bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := io.WriteString(output, "{\"schema_version\":1,\"packets\":[\n"); err != nil {
		return fmt.Errorf("writing export header: %w", err)
	}

	encoder := NewEncoder(output)
	count := 0
	err := StreamRecords(ctx, open, func(record Record) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !includeRaw {
			record = recordWithoutRaw(record)
		}
		if count > 0 {
			if _, err := io.WriteString(output, ","); err != nil {
				return fmt.Errorf("writing packet separator: %w", err)
			}
		}
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("writing packet record: %w", err)
		}
		count++
		return nil
	})
	// StreamRecords intentionally treats context cancellation as a clean stop.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "],\"packet_count\":%d}\n", count); err != nil {
		return fmt.Errorf("writing export footer: %w", err)
	}
	return ctx.Err()
}

func recordWithoutRaw(record Record) Record {
	record.RawHex = ""
	record.PayloadPreview = ""
	if record.Inner != nil {
		inner := *record.Inner
		record.Inner = &inner
		inner.RawHex = ""
		inner.PayloadHex = ""
		if inner.TCP != nil {
			tcp := *inner.TCP
			inner.TCP = &tcp
			tcp.PaddingHex = ""
			tcp.PayloadHex = ""
		}
		if inner.UDP != nil {
			udp := *inner.UDP
			inner.UDP = &udp
			udp.PayloadHex = ""
		}
		if inner.ICMPv4 != nil {
			icmp := *inner.ICMPv4
			inner.ICMPv4 = &icmp
			icmp.PayloadHex = ""
		}
		if inner.ICMPv6 != nil {
			icmp := *inner.ICMPv6
			inner.ICMPv6 = &icmp
			icmp.PayloadHex = ""
		}
		if inner.DNS != nil {
			dns := *inner.DNS
			inner.DNS = &dns
			dns.RawHex = ""
		}
	}
	if record.DiscoMeta != nil {
		disco := *record.DiscoMeta
		record.DiscoMeta = &disco
		if disco.Frame != nil {
			frame := *disco.Frame
			disco.Frame = &frame
			frame.Trailing = ""
		}
	}
	return record
}
