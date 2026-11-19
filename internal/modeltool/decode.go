package modeltool

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

const maxBytes = 32 << 10
const maxDepth = 16

type valueSize struct {
	nodes int
	text  int
}

// decode rejects duplicate keys, invalid unicode and oversized json trees.
func decode(raw []byte, limit int) (any, valueSize, error) {
	var size valueSize
	if len(raw) > maxBytes {
		return nil, size, errors.New("json exceeds 32 kib")
	}
	if !utf8.Valid(raw) {
		return nil, size, errors.New("json contains invalid utf-8")
	}
	if err := checkStrings(raw); err != nil {
		return nil, size, err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	value, err := decodeValue(decoder, 1, limit, &size)
	if err != nil {
		return nil, size, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, size, errors.New("json must contain exactly one value")
	}
	return value, size, nil
}

// decodeValue counts values and object keys while decoding a bounded tree.
func decodeValue(decoder *json.Decoder, depth, limit int, size *valueSize) (any, error) {
	size.nodes++
	if depth > maxDepth || size.nodes > limit {
		return nil, errors.New("json exceeds depth or node limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, errors.New("invalid json")
	}
	if text, ok := token.(string); ok {
		size.text += len(text)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}

	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			token, err := decoder.Token()
			key, ok := token.(string)
			if err != nil || !ok {
				return nil, errors.New("invalid json object key")
			}
			size.nodes++
			size.text += len(key)
			if _, found := object[key]; found {
				return nil, errors.New("json contains a duplicate object key")
			}
			value, err := decodeValue(decoder, depth+1, limit, size)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, errors.New("invalid json object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeValue(decoder, depth+1, limit, size)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, errors.New("invalid json array")
		}
		return array, nil
	default:
		return nil, errors.New("unexpected json delimiter")
	}
}

// checkStrings rejects unpaired surrogate escapes before encoding/json replaces them.
func checkStrings(raw []byte) error {
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) || raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return errors.New("invalid json unicode escape")
		}
		value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return errors.New("invalid json unicode escape")
		}
		i += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return errors.New("json contains an unpaired unicode surrogate")
		}
		if value < 0xd800 || value > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return errors.New("json contains an unpaired unicode surrogate")
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return errors.New("json contains an unpaired unicode surrogate")
		}
		i += 6
	}
	return nil
}
