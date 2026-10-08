package config

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"pitstop/internal/model"
)

// Parse strictly decodes one supported configuration document.
func Parse(data []byte) (model.Config, error) {
	if len(data) > model.MaxConfigBytes {
		return model.Config{}, invalid("configuration exceeds the size limit")
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return model.Config{}, invalid("configuration must not contain a BOM")
	}
	if !utf8.Valid(data) {
		return model.Config{}, invalid("configuration must be valid UTF-8")
	}
	if err := validateEscapedSurrogates(data); err != nil {
		return model.Config{}, invalid("configuration contains invalid Unicode escapes")
	}
	if err := preflight(data); err != nil {
		return model.Config{}, invalid("invalid configuration JSON")
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return model.Config{}, invalid("invalid configuration JSON")
	}
	version, present, err := schemaVersion(root["schemaVersion"])
	if err != nil {
		return model.Config{}, invalid("schemaVersion must be an integer")
	}
	if !present {
		return model.Config{}, invalid("schemaVersion is required")
	}
	if version == 1 {
		return model.Config{}, model.Err("CONFIG_INVALID", "schemaVersion 1 act workflows are no longer supported; migrate to schemaVersion 2 Docker checks", nil)
	}
	if version != model.ConfigSchemaVersion {
		return model.Config{}, invalid("unsupported config schemaVersion")
	}
	if err := validateV2Shape(root); err != nil {
		return model.Config{}, invalid("invalid configuration shape")
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c model.Config
	if err := dec.Decode(&c); err != nil {
		return model.Config{}, invalid("invalid configuration shape")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return model.Config{}, invalid("configuration must contain one JSON document")
	}
	if err := c.Validate(); err != nil {
		return model.Config{}, err
	}
	return c, nil
}

func invalid(message string) error { return model.Err("CONFIG_INVALID", message, nil) }

func readConfig(repo string) (data []byte, err error) {
	root, err := os.OpenRoot(repo)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.OpenFile(".pitstop.json", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, os.ErrInvalid
	}
	data, err = io.ReadAll(io.LimitReader(f, int64(model.MaxConfigBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > model.MaxConfigBytes {
		return nil, os.ErrInvalid
	}
	return data, nil
}

func schemaVersion(raw json.RawMessage) (int, bool, error) {
	if len(raw) == 0 {
		return 0, false, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return 0, true, err
	}
	n, ok := tok.(json.Number)
	if !ok || n.String() == "" || strings.Trim(n.String(), "0123456789") != "" {
		return 0, true, io.ErrUnexpectedEOF
	}
	v, err := strconv.ParseInt(n.String(), 10, 32)
	if err != nil {
		return 0, true, err
	}
	return int(v), true, nil
}

func preflight(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	root, ok := tok.(json.Delim)
	if !ok || root != '{' {
		return io.ErrUnexpectedEOF
	}
	if err := walkObject(dec, 1); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func walkValue(dec *json.Decoder, depth int) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if depth > model.MaxJSONDepth {
		return io.ErrUnexpectedEOF
	}
	switch delim {
	case '{':
		return walkObject(dec, depth)
	case '[':
		return walkArray(dec, depth)
	default:
		return io.ErrUnexpectedEOF
	}
	return nil
}

func walkObject(dec *json.Decoder, depth int) error {
	if depth > model.MaxJSONDepth {
		return io.ErrUnexpectedEOF
	}
	seen := make(map[string]struct{})
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}
		name, ok := key.(string)
		if !ok {
			return io.ErrUnexpectedEOF
		}
		if depth == 1 && strings.EqualFold(name, "schemaVersion") && name != "schemaVersion" {
			return io.ErrUnexpectedEOF
		}
		if _, exists := seen[name]; exists {
			return io.ErrUnexpectedEOF
		}
		seen[name] = struct{}{}
		if err := walkValue(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim('}') {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func walkArray(dec *json.Decoder, depth int) error {
	if depth > model.MaxJSONDepth {
		return io.ErrUnexpectedEOF
	}
	for dec.More() {
		if err := walkValue(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim(']') {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func validateV2Shape(root map[string]json.RawMessage) error {
	if err := checkObjectFields(root, map[string]struct{}{
		"schemaVersion": {}, "runner": {}, "jobs": {}, "checks": {},
	}, map[string]struct{}{
		"schemaVersion": {}, "runner": {}, "jobs": {}, "checks": {},
	}); err != nil {
		return err
	}
	if _, err := strictObject(root["runner"], map[string]struct{}{
		"image": {}, "architecture": {},
	}, map[string]struct{}{
		"image": {}, "architecture": {},
	}); err != nil {
		return err
	}
	checks, err := rawArray(root["checks"])
	if err != nil {
		return err
	}
	for _, checkRaw := range checks {
		check, err := strictObject(checkRaw, map[string]struct{}{
			"id": {}, "steps": {},
		}, map[string]struct{}{
			"id": {}, "steps": {},
		})
		if err != nil {
			return err
		}
		steps, err := rawArray(check["steps"])
		if err != nil {
			return err
		}
		for _, stepRaw := range steps {
			if _, err := strictObject(stepRaw, map[string]struct{}{
				"id": {}, "name": {}, "run": {},
			}, map[string]struct{}{
				"id": {}, "run": {},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkObjectFields(object map[string]json.RawMessage, allowed, required map[string]struct{}) error {
	if object == nil {
		return io.ErrUnexpectedEOF
	}
	for name := range object {
		if _, ok := allowed[name]; !ok {
			return io.ErrUnexpectedEOF
		}
		if bytes.Equal(bytes.TrimSpace(object[name]), []byte("null")) {
			return io.ErrUnexpectedEOF
		}
	}
	for name := range required {
		if _, ok := object[name]; !ok {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

func strictObject(raw json.RawMessage, allowed, required map[string]struct{}) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if err := checkObjectFields(object, allowed, required); err != nil {
		return nil, err
	}
	return object, nil
}

func rawArray(raw json.RawMessage) ([]json.RawMessage, error) {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, io.ErrUnexpectedEOF
	}
	return values, nil
}

func validateEscapedSurrogates(data []byte) error {
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		for i++; i < len(data); i++ {
			switch data[i] {
			case '"':
				goto nextString
			case '\\':
				i++
				if i >= len(data) {
					return io.ErrUnexpectedEOF
				}
				if data[i] != 'u' {
					continue
				}
				if i+4 >= len(data) {
					return io.ErrUnexpectedEOF
				}
				code, ok := hex4(data[i+1 : i+5])
				if !ok {
					return io.ErrUnexpectedEOF
				}
				i += 4
				if code >= 0xdc00 && code <= 0xdfff {
					return io.ErrUnexpectedEOF
				}
				if code >= 0xd800 && code <= 0xdbff {
					if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
						return io.ErrUnexpectedEOF
					}
					low, ok := hex4(data[i+3 : i+7])
					if !ok || low < 0xdc00 || low > 0xdfff {
						return io.ErrUnexpectedEOF
					}
					i += 6
				}
			}
		}
		return io.ErrUnexpectedEOF
	nextString:
	}
	return nil
}

func hex4(b []byte) (int, bool) {
	if len(b) != 4 {
		return 0, false
	}
	n := 0
	for _, c := range b {
		n <<= 4
		switch {
		case c >= '0' && c <= '9':
			n += int(c - '0')
		case c >= 'a' && c <= 'f':
			n += int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			n += int(c-'A') + 10
		default:
			return 0, false
		}
	}
	return n, true
}
