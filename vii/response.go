package vii

import (
	"encoding/json"
	"net/http"
)

//=====================================
// response helpers
//=====================================

// WriteError sends a JSON error message with a given status code.
func WriteError(w http.ResponseWriter, statusCode int, message string) error {
	return WriteJSON(w, statusCode, map[string]string{"error": message})
}

// Redirect is a convenience wrapper for http.Redirect.
func Redirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	http.Redirect(w, r, url, code)
}

// SetHeader is a shortcut for w.Header().Set(key, value).
func SetHeader(w http.ResponseWriter, key, value string) {
	w.Header().Set(key, value)
}

// SetCookie adds a Set-Cookie header to the provided ResponseWriter's headers.
func SetCookie(w http.ResponseWriter, cookie *http.Cookie) {
	http.SetCookie(w, cookie)
}

// WriteHTML writes a raw HTML string as the response.
func WriteHTML(w http.ResponseWriter, status int, content string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	w.Write([]byte(content))
}

// WriteText writes a plain text string as the response.
func WriteText(w http.ResponseWriter, status int, content string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(content))
}

// WriteJSON serializes the data interface to JSON and writes it as the response.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		// Don't write the error to the response here, as the header is already written.
		// The caller should handle the error.
		return err
	}
	return nil
}
