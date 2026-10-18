package normalize

import "encoding/json"

func isOne(v any) bool {
	switch n := v.(type) {
	case float64:
		return n == 1
	case int:
		return n == 1
	case int64:
		return n == 1
	case json.Number:
		i, err := n.Int64()
		return err == nil && i == 1
	default:
		return false
	}
}

func numEq(v any, want int64) bool {
	switch n := v.(type) {
	case float64:
		return n == float64(want)
	case int:
		return int64(n) == want
	case int64:
		return n == want
	case json.Number:
		i, err := n.Int64()
		return err == nil && i == want
	default:
		return false
	}
}
