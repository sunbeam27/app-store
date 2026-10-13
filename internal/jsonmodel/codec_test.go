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
		{"9223372036854775807.99", "9223372036854775807"},
		{"-9223372036854775808.99", "-9223372036854775808"},
		{"1e-1000000", "0"},
		{"-1e-1000000", "0"},
		{"0e1000000", "0"},
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
	for _, number := range []string{"1e50", "1e1000000", "-1e1000000", "9223372036854775808.0", "-9223372036854775809.0"} {
		if _, err := NormalizeTimestamps([]byte(`{"date":`+number+`}`), []string{"date"}); err == nil {
			t.Fatalf("overflow accepted: %s", number)
		}
	}
}
