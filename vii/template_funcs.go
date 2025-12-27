package vii

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode"
)

func TemplateFuncsCommon() template.FuncMap {
	return template.FuncMap{
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"title":      tmplTitle, // avoid removed strings.Title
		"trim":       strings.TrimSpace,
		"printf":     fmt.Sprintf,
		"dict":       tmplDict,
		"get":        tmplGet,
		"default":    tmplDefault,
		"now":        time.Now,
		"formatTime": tmplFormatTime,
		"json":       tmplJSON,
		"safeHTML":   tmplSafeHTML,
	}
}

func tmplSafeHTML(s string) template.HTML { return template.HTML(s) }

func tmplJSON(v any) template.HTML {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return template.HTML(b)
}

func tmplTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	for i := range parts {
		w := strings.ToLower(parts[i])
		r := []rune(w)
		if len(r) == 0 {
			continue
		}
		r[0] = unicode.ToTitle(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

func tmplDict(kv ...any) map[string]any {
	if len(kv) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok || k == "" {
			continue
		}
		out[k] = kv[i+1]
	}
	return out
}

func tmplGet(m map[string]any, key string) any {
	if m == nil || key == "" {
		return nil
	}
	return m[key]
}

func tmplDefault(fallback any, v any) any {
	if v == nil {
		return fallback
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return fallback
		}
		return x
	case bool:
		if x == false {
			return fallback
		}
		return x
	default:
		return v
	}
}

func tmplFormatTime(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	if strings.TrimSpace(layout) == "" {
		return t.Format(time.RFC3339)
	}
	return t.Format(layout)
}