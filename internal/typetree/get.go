package typetree

func Str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func Int(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case bool:
		if v {
			return 1
		}
	case float64:
		return int64(v)
	}
	return 0
}

func Float(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	}
	return 0
}

func Bytes(m map[string]any, key string) []byte {
	b, _ := m[key].([]byte)
	return b
}

func Map(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func List(m map[string]any, key string) []map[string]any {
	items, _ := m[key].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if v, ok := it.(map[string]any); ok {
			out = append(out, v)
		}
	}
	return out
}

func Strings(m map[string]any, key string) []string {
	items, _ := m[key].([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
