package jsonmodel

import (
	"encoding/json"
	"testing"
)

func TestExactTimestampTruncation(t *testing.T) {
	for _, test := range []struct{ number, want string }{
		{"1697679936049.7297", "1697679936049"},
		{"-62135769600000.123", "-62135769600000"},
		{"9.007199254740993e15", "9007199254740993"},
	} {
		result, err := NormalizeTimestamps([]byte(`{"date":`+test.number+`}`), []string{"date"})
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		json.Unmarshal(result, &fields)
		if string(fields["date"]) != test.want {
			t.Fatalf("%s: %s", test.number, result)
		}
	}
	if _, err := NormalizeTimestamps([]byte(`{"date":1e50}`), []string{"date"}); err == nil {
		t.Fatal("overflow accepted")
	}
}
