package explainauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// DecodeAuthorizationRequest performs strict JSON decoding, including nested
// duplicate-key rejection and exact qrl_signMessage field widths.
func DecodeAuthorizationRequest(data []byte) (AuthorizationRequest, error) {
	var request AuthorizationRequest
	if len(bytes.TrimSpace(data)) == 0 {
		return request, fmt.Errorf("authorization body is empty")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return request, err
	}
	if err := requireExactAuthorizationKeys(data); err != nil {
		return request, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, fmt.Errorf("decode authorization body: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return request, err
	}
	if err := validateAuthorizationEncoding(request); err != nil {
		return request, err
	}
	return request, nil
}

func requireExactAuthorizationKeys(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("decode authorization object: %w", err)
	}
	if err := requireExactObjectKeys(root, "authorization", "challengeId", "proof"); err != nil {
		return err
	}

	var proof map[string]json.RawMessage
	if err := json.Unmarshal(root["proof"], &proof); err != nil {
		return fmt.Errorf("decode authorization proof object: %w", err)
	}
	return requireExactObjectKeys(
		proof,
		"authorization proof",
		"signature",
		"publicKey",
		"descriptor",
		"signer",
		"digest",
		"schemeVersion",
	)
}

func requireExactObjectKeys(object map[string]json.RawMessage, label string, expected ...string) error {
	if object == nil {
		return fmt.Errorf("%s must be a JSON object", label)
	}
	allowed := make(map[string]struct{}, len(expected))
	for _, key := range expected {
		allowed[key] = struct{}{}
		if _, ok := object[key]; !ok {
			return fmt.Errorf("%s is missing exact key %q", label, key)
		}
	}
	for key := range object {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%s contains unexpected key %q", label, key)
		}
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("authorization body contains a trailing JSON value")
		}
		return fmt.Errorf("decode trailing authorization data: %w", err)
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON token %v", token)
		}
		return fmt.Errorf("decode trailing JSON token: %w", err)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode JSON token: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("decode JSON object key: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("invalid JSON object terminator")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("invalid JSON array terminator")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}
