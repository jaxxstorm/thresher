package capture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/gopacket/pcapgo"
	"tailscale.com/client/local"
)

type StreamOpener func(context.Context) (io.ReadCloser, error)

type RecordHandler func(Record) error

func OpenLocalAPIStream(ctx context.Context) (io.ReadCloser, error) {
	var client local.Client
	stream, err := client.StreamDebugCapture(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting debug capture stream: %w", err)
	}
	return stream, nil
}

func SelectOutputWriter(outputPath string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if outputPath == "" || outputPath == "-" {
		return stdout, nopWriteCloser{}, nil
	}

	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("opening output file: %w", err)
	}

	return f, f, nil
}

func StreamRecords(ctx context.Context, open StreamOpener, handle RecordHandler) error {
	stream, err := open(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	defer stream.Close()

	var closeOnce sync.Once
	closeStream := func() {
		closeOnce.Do(func() {
			_ = stream.Close()
		})
	}
	defer closeStream()

	go func() {
		<-ctx.Done()
		closeStream()
	}()

	reader, err := pcapgo.NewReader(stream)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("reading capture stream header: %w", err)
	}

	analyzer := NewAnalyzer()
	frame := 0
	for {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		data, ci, err := reader.ReadPacketData()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("reading packet data: %w", err)
		}

		frame++
		record, decodeErr := DecodePacket(frame, ci.Timestamp, data)
		if decodeErr != nil {
			record = RecordDecodeErrorFrom(record, decodeErr)
		} else {
			record = analyzer.Analyze(record)
		}

		if err := handle(record); err != nil {
			return err
		}
	}
}

func StreamJSONL(ctx context.Context, output io.Writer, open StreamOpener, format OutputFormat) error {
	writer, err := NewRecordWriter(output, format)
	if err != nil {
		return err
	}

	return StreamRecords(ctx, open, func(record Record) error {
		if err := writer.WriteRecord(record); err != nil {
			return fmt.Errorf("writing packet record: %w", err)
		}
		return nil
	})
}

type nopWriteCloser struct{}

func (nopWriteCloser) Close() error { return nil }
