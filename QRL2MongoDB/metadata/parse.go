package metadata

import (
	"QRL2MongoDB/models"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// metadataDocument is the minimal subset of the NFT metadata schema we
// care about for Phase 3a (collection level). Unknown fields are ignored.
// Each field is optional; the parser coerces wrong-typed fields to the
// empty string so one bad entry can't fail the whole record.
type metadataDocument struct {
	Name        string `json:"-"`
	Description string `json:"-"`
	Image       string `json:"-"`
	ExternalURL string `json:"-"`
}

// tokenMetadataDocument extends metadataDocument with the per-token
// `attributes` array (Phase 3b). Values are coerced to strings for
// storage uniformity since the OpenSea spec allows mixed types
// (numeric, string, object) and downstream consumers can re-parse.
type tokenMetadataDocument struct {
	Name        string
	Description string
	Image       string
	ExternalURL string
	Attributes  []models.TokenAttribute
}

// safeExternalURL returns the trimmed value only when it carries an http(s)
// scheme. Anything else (javascript:, data:, relative paths, etc.) is dropped
// to an empty string so an attacker-controlled contractURI/tokenURI cannot
// persist a hostile external_url. Defense-in-depth alongside frontend escaping.
func safeExternalURL(s string) string {
	t := strings.TrimSpace(s)
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return t
	}
	return ""
}

// parseMetadataJSON does a defensive JSON parse: unmarshal into a generic
// map, then pull out the string fields we care about, ignoring any non-
// string variants. This lets a malformed `description: ["foo", "bar"]`
// reduce to an empty description rather than failing the entire record.
//
// Two sentinel cases for which we still return an empty doc + nil error
// rather than a JSON-shape error:
//
//   - the JSON literal `null`, which unmarshals into a nil map; reading
//     `raw[key]` from a nil map is safe in Go but the explicit check keeps
//     the intent obvious and silences static analysis warnings;
//   - top-level scalars or arrays, where Unmarshal fails with a type
//     mismatch error - those still fail loudly.
func parseMetadataJSON(body []byte) (metadataDocument, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return metadataDocument{}, err
	}
	stringField := func(key string) string {
		// Reading from a nil map is safe in Go, but the explicit branch
		// avoids any ambiguity for future readers.
		if raw == nil {
			return ""
		}
		v, ok := raw[key]
		if !ok || v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return strings.TrimSpace(s)
	}
	doc := metadataDocument{
		Name:        stringField("name"),
		Description: stringField("description"),
		Image:       stringField("image"),
		ExternalURL: safeExternalURL(stringField("external_url")),
	}
	return doc, nil
}

// parseTokenMetadataJSON is the Phase 3b variant that also extracts the
// `attributes` array (OpenSea convention: each entry has `trait_type`,
// `value`, optionally `display_type`). Anything that isn't a clean array
// of objects is dropped silently, one bad NFT shouldn't fail the rest.
//
// Uses json.Decoder.UseNumber() rather than vanilla json.Unmarshal so
// trait values that are large integers (e.g. token IDs as attribute
// values, common in game NFT schemas) keep their full precision instead
// of being rounded through float64 at 2^53.
func parseTokenMetadataJSON(body []byte) (tokenMetadataDocument, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return tokenMetadataDocument{}, err
	}
	stringField := func(key string) string {
		if raw == nil {
			return ""
		}
		v, ok := raw[key]
		if !ok || v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return strings.TrimSpace(s)
	}

	out := tokenMetadataDocument{
		Name:        stringField("name"),
		Description: stringField("description"),
		Image:       stringField("image"),
		ExternalURL: safeExternalURL(stringField("external_url")),
	}

	// attributes: []{trait_type, value, display_type?}
	if raw != nil {
		if attrsAny, ok := raw["attributes"]; ok {
			if attrs, isArr := attrsAny.([]interface{}); isArr {
				const maxAttrs = 64 // defensive: stop a malicious doc from blowing memory
				for i, a := range attrs {
					if i >= maxAttrs {
						break
					}
					m, isMap := a.(map[string]interface{})
					if !isMap {
						continue
					}
					attr := models.TokenAttribute{
						TraitType:   stringifyAttrField(m["trait_type"]),
						Value:       stringifyAttrField(m["value"]),
						DisplayType: stringifyAttrField(m["display_type"]),
					}
					if attr.TraitType == "" && attr.Value == "" {
						continue // empty attr, skip
					}
					out.Attributes = append(out.Attributes, attr)
				}
			}
		}
	}
	return out, nil
}

// stringifyAttrField coerces an arbitrary JSON value to a string for
// storage uniformity. Strings pass through trimmed; numbers (preserved as
// json.Number via Decoder.UseNumber so we keep full precision for big
// uint256-shaped trait values) pass through verbatim; bools become
// "true"/"false"; nil and objects collapse to "".
//
// Older callers may still feed float64 values when the JSON came from
// json.Unmarshal instead of json.Decoder.UseNumber; the float64 branch
// is kept for that compatibility path.
func stringifyAttrField(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		// Decoder.UseNumber path: keep the original textual form, no
		// float64 round-trip. Covers ints past 2^53.
		return t.String()
	case float64:
		// Vanilla Unmarshal fallback; preserve integer-ness when possible.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}
