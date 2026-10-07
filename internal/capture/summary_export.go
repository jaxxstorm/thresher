package capture

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// ExportSummaryJSON scans a capture before writing a bounded, lossy summary.
// A write failure may leave partial JSON; input failures produce no output.
func ExportSummaryJSON(ctx context.Context, output io.Writer, open StreamOpener) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s := newSummaryAccumulator()
	var input *summaryInput
	var openErr error
	// Classic PCAP has a 24-byte file header and a 16-byte header per record.
	// pcapgo can return bare EOF when a record header is present but its entire
	// payload is missing; byte accounting distinguishes that from clean EOF.
	expectedBytes := uint64(24)
	err := StreamRecords(ctx, func(ctx context.Context) (io.ReadCloser, error) {
		stream, err := open(ctx)
		openErr = err
		if err != nil {
			return nil, err
		}
		input = &summaryInput{ReadCloser: stream}
		return input, nil
	}, func(record Record) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.add(record)
		expectedBytes += 16 + uint64(record.FrameLength)
		return nil
	})
	if input != nil && input.gzip != nil {
		defer input.gzip.Close()
	}
	// StreamRecords treats cancellation as a clean live-capture stop, but an
	// offline export must not publish a partial scan, even for reader cancellation.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	if openErr != nil {
		return openErr
	}
	if input != nil && input.err != nil {
		return fmt.Errorf("reading summary input: %w", input.err)
	}
	if input != nil && input.bytes != expectedBytes {
		return fmt.Errorf("reading summary input: %w", io.ErrUnexpectedEOF)
	}
	doc := s.finalize()
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding summary: %w", err)
	}
	data = append(data, '\n')
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := output.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}
	return ctx.Err()
}

type summaryInput struct {
	io.ReadCloser
	reader io.Reader
	gzip   *gzip.Reader
	err    error
	bytes  uint64
}

func (r *summaryInput) Read(p []byte) (int, error) {
	if r.reader == nil {
		// Initialize lazily so StreamRecords can close the original stream on
		// cancellation during header reads. Count PCAP bytes, not gzip bytes.
		buffered := bufio.NewReader(r.ReadCloser)
		magic, err := buffered.Peek(2)
		if err != nil {
			r.err = err
			return 0, err
		}
		r.reader = buffered
		if magic[0] == 0x1f && magic[1] == 0x8b {
			r.gzip, err = gzip.NewReader(buffered)
			if err != nil {
				r.err = err
				return 0, err
			}
			r.reader = r.gzip
		}
	}
	n, err := r.reader.Read(p)
	r.bytes += uint64(n)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}
