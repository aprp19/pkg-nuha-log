package accesslog

import "strings"

var sensitiveParamKeys = map[string]struct{}{
	"password":       {},
	"token":          {},
	"access_token":   {},
	"refresh_token":  {},
	"authorization":  {},
	"access_code":    {},
	"document_token": {},
}

func isSensitiveKey(key string) bool {
	_, ok := sensitiveParamKeys[strings.ToLower(key)]
	return ok
}

func sanitizeValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = sanitizeValue(item)
		}
		return out
	case map[string]interface{}:
		return sanitizeObject(v)
	default:
		return value
	}
}

func sanitizeObject(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(obj))
	for key, value := range obj {
		if isSensitiveKey(key) {
			result[key] = "[REDACTED]"
			continue
		}
		result[key] = sanitizeValue(value)
	}
	return result
}

func sanitizeResponseBody(body []byte, maxBytes int) interface{} {
	if len(body) == 0 {
		return nil
	}

	truncated := false
	if len(body) > maxBytes {
		body = body[:maxBytes]
		truncated = true
	}

	var parsed interface{}
	if err := jsonUnmarshal(body, &parsed); err != nil {
		text := string(body)
		if truncated {
			return map[string]interface{}{
				"_truncated": true,
				"_raw":       text,
			}
		}
		return text
	}

	sanitized := sanitizeValue(parsed)
	if truncated {
		if obj, ok := sanitized.(map[string]interface{}); ok {
			obj["_truncated"] = true
			return obj
		}
		return map[string]interface{}{
			"_truncated": true,
			"data":       sanitized,
		}
	}
	return sanitized
}
