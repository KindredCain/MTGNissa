package httpapi

import (
	"encoding/json"
	"io"
)

func newJSONDecoder(r io.Reader) *json.Decoder { return json.NewDecoder(r) }
