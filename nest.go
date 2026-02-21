package wideevent

import "strings"

// nestFields converts a flat slice of fields with dot-separated keys
// into a nested map structure suitable for JSON serialization.
// For example, "db.operation" = "update" becomes {"db": {"operation": "update"}}.
// Later values for the same key overwrite earlier ones.
func nestFields(fields []Field) map[string]any {
	root := make(map[string]any)
	for _, f := range fields {
		parts := strings.Split(f.Key, ".")
		if len(parts) == 1 {
			root[f.Key] = f.Value
			continue
		}
		current := root
		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = f.Value
			} else {
				next, ok := current[part]
				if !ok {
					m := make(map[string]any)
					current[part] = m
					current = m
				} else if m, ok := next.(map[string]any); ok {
					current = m
				} else {
					// Key conflict: intermediate is not a map; overwrite
					m := make(map[string]any)
					current[part] = m
					current = m
				}
			}
		}
	}
	return root
}
