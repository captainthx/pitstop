package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"pitstop/internal/model"
)

// Parse strictly decodes one supported configuration document.
func Parse(data []byte) (model.Config, error) {
	if len(data) > model.MaxConfigBytes {
		return model.Config{}, issue("SIZE_LIMIT", "", "configuration exceeds the size limit")
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return model.Config{}, issue("JSON_INVALID", "", "configuration must not contain a BOM")
	}
	if !utf8.Valid(data) {
		return model.Config{}, issue("JSON_INVALID", "", "configuration must be valid UTF-8")
	}
	if err := validateEscapedSurrogates(data); err != nil {
		return model.Config{}, issue("JSON_INVALID", "", "configuration contains invalid Unicode escapes")
	}
	if err := preflight(data); err != nil {
		if pe, ok := err.(parseIssue); ok {
			return model.Config{}, issue(pe.code, pe.path, pe.message)
		}
		return model.Config{}, issue("JSON_INVALID", "", "invalid configuration JSON")
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return model.Config{}, issue("JSON_INVALID", "", "invalid configuration JSON")
	}
	version, present, err := schemaVersion(root["schemaVersion"])
	if err != nil {
		return model.Config{}, issue("JSON_TYPE", "/schemaVersion", "schemaVersion must be an integer")
	}
	if !present {
		return model.Config{}, issue("FIELD_REQUIRED", "/schemaVersion", "schemaVersion is required")
	}
	if version == 1 {
		message := "schemaVersion 1 act workflows are no longer supported; migrate to schemaVersion 2 Docker checks"
		return model.Config{}, model.Err("CONFIG_INVALID", message, &model.ValidationError{Issue: model.ConfigIssue{Code: "SCHEMA_VERSION", Path: "/schemaVersion", Message: message}})
	}
	if version != model.ConfigSchemaVersion {
		return model.Config{}, issue("SCHEMA_VERSION", "/schemaVersion", "unsupported config schemaVersion")
	}
	if err := validateV2ShapeOrdered(data); err != nil {
		return model.Config{}, err
	}
	if err := validateV2Shape(root); err != nil {
		return model.Config{}, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c model.Config
	if err := dec.Decode(&c); err != nil {
		return model.Config{}, issue("JSON_TYPE", "", "invalid configuration shape")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return model.Config{}, issue("JSON_INVALID", "", "configuration must contain one JSON document")
	}
	if err := c.Validate(); err != nil {
		return model.Config{}, err
	}
	return c, nil
}

func issue(code, path, message string) error {
	return model.Err("CONFIG_INVALID", message, &model.ValidationError{Issue: model.ConfigIssue{Code: code, Path: path, Message: message}})
}

type parseIssue struct{ code, path, message string }

func (e parseIssue) Error() string { return e.message }

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
		return nil, errConfigSize
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
			return parseIssue{code: "JSON_UNKNOWN_FIELD", path: safePath("", name), message: "unknown configuration field"}
		}
		if _, exists := seen[name]; exists {
			return parseIssue{code: "JSON_DUPLICATE_KEY", path: "", message: "configuration contains duplicate object keys"}
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
	for _, field := range []string{"schemaVersion", "runner", "jobs", "checks"} {
		if _, ok := root[field]; !ok {
			return issue("FIELD_REQUIRED", safePath("", field), "required configuration field is missing")
		}
	}
	runner, err := objectField(root["runner"], "/runner")
	if err != nil {
		return err
	}
	if err := requiredField(runner, "image", "/runner/image", "string"); err != nil {
		return err
	}
	if err := requiredField(runner, "architecture", "/runner/architecture", "string"); err != nil {
		return err
	}
	if err := ensureType(root["jobs"], "/jobs", "array"); err != nil {
		return err
	}
	jobs, _ := rawArray(root["jobs"])
	for i, raw := range jobs {
		if err := ensureType(raw, "/jobs/"+strconv.Itoa(i), "string"); err != nil {
			return err
		}
	}
	checks, err := rawArray(root["checks"])
	if err != nil {
		return issue("JSON_TYPE", "/checks", "checks must be an array")
	}
	for i, checkRaw := range checks {
		checkPath := "/checks/" + strconv.Itoa(i)
		check, err := objectField(checkRaw, checkPath)
		if err != nil {
			return err
		}
		if err := requiredField(check, "id", checkPath+"/id", "string"); err != nil {
			return err
		}
		if err := requiredField(check, "steps", checkPath+"/steps", "array"); err != nil {
			return err
		}
		steps, _ := rawArray(check["steps"])
		for j, stepRaw := range steps {
			stepPath := checkPath + "/steps/" + strconv.Itoa(j)
			step, err := objectField(stepRaw, stepPath)
			if err != nil {
				return err
			}
			if err := requiredField(step, "id", stepPath+"/id", "string"); err != nil {
				return err
			}
			if err := requiredField(step, "run", stepPath+"/run", "string"); err != nil {
				return err
			}
			if raw, ok := step["name"]; ok {
				if err := ensureType(raw, stepPath+"/name", "string"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func objectField(raw json.RawMessage, path string) (map[string]json.RawMessage, error) {
	if err := ensureType(raw, path, "object"); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, issue("JSON_TYPE", path, "field must be an object")
	}
	return object, nil
}

func requiredField(object map[string]json.RawMessage, name, path, kind string) error {
	raw, ok := object[name]
	if !ok {
		return issue("FIELD_REQUIRED", path, "required configuration field is missing")
	}
	return ensureType(raw, path, kind)
}

func ensureType(raw json.RawMessage, path, kind string) error {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return issue("JSON_TYPE", path, "field has an invalid type")
	}
	var ok bool
	switch kind {
	case "object":
		var value map[string]json.RawMessage
		ok = json.Unmarshal(raw, &value) == nil
	case "array":
		var value []json.RawMessage
		ok = json.Unmarshal(raw, &value) == nil
	case "string":
		var value string
		ok = json.Unmarshal(raw, &value) == nil
	default:
		ok = false
	}
	if !ok {
		return issue("JSON_TYPE", path, "field has an invalid type")
	}
	return nil
}

func validateV2ShapeOrdered(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return issue("JSON_INVALID", "", "invalid configuration JSON")
	}
	return scanObject(dec, tok, "", map[string]struct{}{"schemaVersion": {}, "runner": {}, "jobs": {}, "checks": {}}, func(key string, value any, path string) (bool, error) {
		switch key {
		case "runner":
			return true, scanObject(dec, value, path, map[string]struct{}{"image": {}, "architecture": {}}, nil)
		case "checks":
			return true, scanChecks(dec, value, path)
		default:
			return false, nil
		}
	})
}

func scanChecks(dec *json.Decoder, value any, path string) error {
	open, ok := value.(json.Delim)
	if !ok || open != '[' {
		return nil
	}
	for i := 0; dec.More(); i++ {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		checkPath := path + "/" + strconv.Itoa(i)
		if err := scanObject(dec, tok, checkPath, map[string]struct{}{"id": {}, "steps": {}}, func(key string, value any, fieldPath string) (bool, error) {
			if key != "steps" {
				return false, nil
			}
			return true, scanSteps(dec, value, fieldPath)
		}); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim(']') {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func scanSteps(dec *json.Decoder, value any, path string) error {
	open, ok := value.(json.Delim)
	if !ok || open != '[' {
		return nil
	}
	for i := 0; dec.More(); i++ {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		stepPath := path + "/" + strconv.Itoa(i)
		if err := scanObject(dec, tok, stepPath, map[string]struct{}{"id": {}, "name": {}, "run": {}}, func(key string, value any, fieldPath string) (bool, error) {
			if key == "name" {
				if name, ok := value.(string); ok && name == "" {
					return false, issue("FIELD_INVALID", fieldPath, "step.name is invalid")
				}
			}
			return false, nil
		}); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim(']') {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func scanObject(dec *json.Decoder, opening any, path string, allowed map[string]struct{}, nested func(string, any, string) (bool, error)) error {
	open, ok := opening.(json.Delim)
	if !ok || open != '{' {
		return nil
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return io.ErrUnexpectedEOF
		}
		fieldPath := safePath(path, key)
		if _, ok := allowed[key]; !ok {
			return issue("JSON_UNKNOWN_FIELD", fieldPath, "unknown configuration field")
		}
		value, err := dec.Token()
		if err != nil {
			return err
		}
		consumed := false
		if nested != nil {
			consumed, err = nested(key, value, fieldPath)
			if err != nil {
				return err
			}
		}
		if !consumed {
			if err := skipToken(dec, value); err != nil {
				return err
			}
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim('}') {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func skipToken(dec *json.Decoder, tok any) error {
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return io.ErrUnexpectedEOF
	}
	for dec.More() {
		if delim == '{' {
			if _, err := dec.Token(); err != nil {
				return err
			}
		}
		value, err := dec.Token()
		if err != nil {
			return err
		}
		if err := skipToken(dec, value); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func safePath(base, key string) string {
	var b strings.Builder
	b.WriteString(base)
	b.WriteByte('/')
	for _, r := range key {
		switch {
		case r == '~':
			b.WriteString("~0")
		case r == '/':
			b.WriteString("~1")
		case unicode.IsControl(r) || unicode.In(r, unicode.Cf):
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	path := b.String()
	if len(path) > model.MaxIssuePathBytes {
		path = path[:model.MaxIssuePathBytes]
		for !utf8.ValidString(path) {
			path = path[:len(path)-1]
		}
	}
	return path
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
