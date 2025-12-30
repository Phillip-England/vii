package vii

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

//=====================================
// request helpers
//=====================================

// ReadJSON decodes the JSON body of a request into the provided interface.
// It returns an error if the body is empty, malformed, or too large.
func ReadJSON(r *http.Request, v interface{}) error {
	// Set a max body size to prevent malicious attacks
	maxBytes := 1_048_576 // 1MB
	r.Body = http.MaxBytesReader(nil, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(v)
	if err != nil {
		// Handle specific JSON-related errors
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	// Ensure the body is only read once.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// Header returns the value of a request header.
func Header(r *http.Request, key string) string {
	return r.Header.Get(key)
}

// Cookie returns the named cookie provided in the request.
func Cookie(r *http.Request, name string) (*http.Cookie, error) {
	return r.Cookie(name)
}

// Query returns the value of a URL query parameter.
func Query(r *http.Request, paramName string) string {
	return r.URL.Query().Get(paramName)
}

// QueryIs checks if a URL query parameter equals a specific value.
func QueryIs(r *http.Request, paramName string, valueToCheck string) bool {
	return r.URL.Query().Get(paramName) == valueToCheck
}
