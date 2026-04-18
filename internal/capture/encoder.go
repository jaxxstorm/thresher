package capture

import (
	"encoding/json"
	"io"
)

type Encoder struct {
	enc *json.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &Encoder{enc: enc}
}

func (e *Encoder) Encode(record Record) error {
	return e.enc.Encode(record)
}
