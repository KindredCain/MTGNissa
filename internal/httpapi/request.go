package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"mtgnissa/internal/apperror"
)

type decoderAPI interface {
	Decode(any) error
	DisallowUnknownFields()
}

func jsonDecoder(reader io.Reader) decoderAPI {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	return decoder
}

func ensureEOF(decoder decoderAPI) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected second JSON value")
	}
	return err
}

func requestDecodeError(err error, message string) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return apperror.Wrap(apperror.CodeRequestTooLarge, "request body exceeds the server limit", err)
	}
	return apperror.Wrap(apperror.CodeInvalidRequest, message, err)
}
