// Package jsonmodel implements lossless wire-model JSON handling.
package jsonmodel

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// NormalizeTimestamps mirrors Apple's Xcode-compatible deserializer: decimal
// millisecond values truncate toward zero, with no floating-point precision loss.
func NormalizeTimestamps(data []byte, keys []string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	changed := false
	for _, key := range keys {
		raw, ok := fields[key]
		if !ok || !strings.ContainsAny(string(raw), ".eE") {
			continue
		}
		if len(raw) > 128 {
			return nil, fmt.Errorf("timestamp %s is too large", key)
		}
		rational, ok := new(big.Rat).SetString(string(raw))
		if !ok {
			return nil, fmt.Errorf("invalid timestamp %s", key)
		}
		integer := new(big.Int).Quo(rational.Num(), rational.Denom())
		if !integer.IsInt64() {
			return nil, fmt.Errorf("timestamp %s exceeds int64", key)
		}
		fields[key] = json.RawMessage(integer.String())
		changed = true
	}
	if !changed {
		return data, nil
	}
	return json.Marshal(fields)
}

func Marshal(value any, unknown map[string]json.RawMessage, defaults map[string]string) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for key, value := range defaults {
		if raw, ok := fields[key]; !ok || string(raw) == `""` || string(raw) == "null" {
			fields[key], _ = json.Marshal(value)
		}
	}
	for key, value := range unknown {
		if _, exists := fields[key]; !exists {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}

func Unmarshal(data []byte, value any, known []string) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(data, value); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, key := range known {
		delete(fields, key)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return fields, nil
}
