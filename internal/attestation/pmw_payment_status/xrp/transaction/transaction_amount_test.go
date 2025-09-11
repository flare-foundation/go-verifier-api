package transaction

import "testing"

func TestGetStringField(t *testing.T) {
	m := map[string]interface{}{
		"key1": "val1",
		"key2": 1234,
	}
	t.Run("valid string field", func(t *testing.T) {
		val, ok := getStringField(m, "key1")
		if !ok || val != "val1" {
			t.Fatal("GetStringField failed to get existing string value")
		}
	})
	t.Run("number field", func(t *testing.T) {
		_, ok := getStringField(m, "key2")
		if ok {
			t.Fatal("GetStringField should return false for non-string value")
		}
	})
	t.Run("missing field", func(t *testing.T) {
		_, ok := getStringField(m, "missing")
		if ok {
			t.Fatal("GetStringField should return false for missing key")
		}
	})
}
