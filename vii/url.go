package vii

import (
	"net/http"
	"net/url"
)

// Values is a map of key-value pairs for building URL query strings.
type Values map[string]string

// URL is a reusable primitive for defining, building, and parsing URLs.
type URL struct {
	Path        string
	QueryParams []string
}

// NewURL creates a new URL definition with a static path.
func NewURL(path string) *URL {
	return &URL{
		Path: path,
	}
}

// WithQuery adds expected query parameter keys to the URL definition.
// It allows for method chaining.
func (u *URL) WithQuery(params ...string) *URL {
	u.QueryParams = append(u.QueryParams, params...)
	return u
}

// Build constructs a URL string with the given query parameter values.
// It correctly encodes the values for URL safety.
func (u *URL) Build(values Values) string {
	if len(u.QueryParams) == 0 || len(values) == 0 {
		return u.Path
	}

	queryParams := url.Values{}
	for _, key := range u.QueryParams {
		if val, ok := values[key]; ok {
			queryParams.Add(key, val)
		}
	}

	queryString := queryParams.Encode()
	if queryString == "" {
		return u.Path
	}

	return u.Path + "?" + queryString
}

// Parse extracts the defined query parameters from an HTTP request's URL.
// It returns a Values map containing the keys and their corresponding values.
func (u *URL) Parse(r *http.Request) Values {
	parsed := make(Values)
	requestQuery := r.URL.Query()

	for _, key := range u.QueryParams {
		parsed[key] = requestQuery.Get(key)
	}

	return parsed
}
