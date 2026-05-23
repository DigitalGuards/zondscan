package synchroniser

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
)

func TestResolveMetadataURI(t *testing.T) {
	gateway := "https://qrlwallet.com/api/ipfs/"
	cidV0 := "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
	// A valid CIDv1 (base32, sha-256, dag-pb) for /ipfs/bafy....
	cidV1 := "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"

	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "https URL returns as-is",
			uri:  "https://example.com/collection.json",
			want: "https://example.com/collection.json",
		},
		{
			name: "http URL returns as-is",
			uri:  "http://nft-host.test/foo.json",
			want: "http://nft-host.test/foo.json",
		},
		{
			name: "ipfs:// CIDv0 + path resolves through gateway",
			uri:  "ipfs://" + cidV0 + "/metadata.json",
			want: gateway + cidV0 + "/metadata.json",
		},
		{
			name: "ipfs:// bare CIDv0 resolves through gateway",
			uri:  "ipfs://" + cidV0,
			want: gateway + cidV0,
		},
		{
			name: "ipfs://ipfs/CID legacy form collapses correctly",
			uri:  "ipfs://ipfs/" + cidV0 + "/img.png",
			want: gateway + cidV0 + "/img.png",
		},
		{
			name: "ipfs:// CIDv1 resolves through gateway",
			uri:  "ipfs://" + cidV1 + "/collection.json",
			want: gateway + cidV1 + "/collection.json",
		},
		{
			name: "bare CIDv0 with path resolves through gateway",
			uri:  cidV0 + "/contract.json",
			want: gateway + cidV0 + "/contract.json",
		},
		{
			name: "bare CIDv0 without path resolves through gateway",
			uri:  cidV0,
			want: gateway + cidV0,
		},
		{
			name:    "empty URI",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "ar:// unsupported scheme",
			uri:     "ar://AR12345-FOO",
			wantErr: true,
		},
		{
			name:    "data: rejected",
			uri:     "data:application/json,{\"name\":\"x\"}",
			wantErr: true,
		},
		{
			name:    "ipfs:// with invalid CID",
			uri:     "ipfs://not-a-cid/path",
			wantErr: true,
		},
		{
			name:    "bare junk",
			uri:     "definitely-not-a-cid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMetadataURI(tt.uri, gateway)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMetadataJSON(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantName    string
		wantDesc    string
		wantImage   string
		wantExtURL  string
		wantErr     bool
	}{
		{
			name:       "OpenSea-style happy path",
			body:       `{"name":"My NFT","description":"It's nice.","image":"ipfs://QmFoo/img.png","external_url":"https://example.com"}`,
			wantName:   "My NFT",
			wantDesc:   "It's nice.",
			wantImage:  "ipfs://QmFoo/img.png",
			wantExtURL: "https://example.com",
		},
		{
			name:     "extra fields ignored",
			body:     `{"name":"Foo","attributes":[{"trait_type":"x","value":"y"}],"image":"ipfs://Q"}`,
			wantName: "Foo",
			wantImage: "ipfs://Q",
		},
		{
			name: "missing fields produce empty strings, not error",
			body: `{}`,
		},
		{
			name:     "wrong-typed description coerces to empty",
			body:     `{"name":"Foo","description":["array","not","string"]}`,
			wantName: "Foo",
		},
		{
			name:     "whitespace trimmed",
			body:     `{"name":"  Trim me  ","description":"\nhi\n"}`,
			wantName: "Trim me",
			wantDesc: "hi",
		},
		{
			name:    "malformed JSON returns error",
			body:    `{"name":"unterminated`,
			wantErr: true,
		},
		{
			name:    "non-object JSON returns error",
			body:    `["just","an","array"]`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMetadataJSON([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantDesc)
			}
			if got.Image != tt.wantImage {
				t.Errorf("Image = %q, want %q", got.Image, tt.wantImage)
			}
			if got.ExternalURL != tt.wantExtURL {
				t.Errorf("ExternalURL = %q, want %q", got.ExternalURL, tt.wantExtURL)
			}
		})
	}
}

func TestParseTokenMetadataJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    tokenMetadataDocument
		wantErr bool
	}{
		{
			// Attributes content is verified field-by-field in the
			// name-keyed assertions below; this `want` only carries the
			// scalar fields for the simple equality cases.
			name: "OpenSea-style happy path with attributes",
			body: `{
				"name":"Sword of Truth #1",
				"description":"Legendary weapon",
				"image":"ipfs://QmFoo/1.png",
				"external_url":"https://example.com/sword/1",
				"attributes":[
					{"trait_type":"Damage","value":50,"display_type":"number"},
					{"trait_type":"Element","value":"Fire"},
					{"trait_type":"Cursed","value":true}
				]
			}`,
		},
		{
			name: "no attributes field",
			body: `{"name":"Plain","image":"ipfs://x"}`,
			want: tokenMetadataDocument{Name: "Plain", Image: "ipfs://x"},
		},
		{
			name: "attributes not an array dropped silently",
			body: `{"name":"X","attributes":"not-an-array"}`,
			want: tokenMetadataDocument{Name: "X"},
		},
		{
			name: "non-object attrs entries skipped",
			body: `{"name":"X","attributes":["string", 42, {"trait_type":"OK","value":"v"}]}`,
			want: tokenMetadataDocument{
				Name: "X",
			},
		},
		{
			name:    "malformed JSON returns error",
			body:    `{"name":`,
			wantErr: true,
		},
		{
			name: "null literal returns empty doc",
			body: `null`,
			want: tokenMetadataDocument{},
		},
		{
			name: "float trait value preserves precision",
			body: `{"attributes":[{"trait_type":"Rarity","value":0.5}]}`,
		},
		{
			// json.Decoder.UseNumber preserves the exact textual form so
			// huge integer trait values (e.g. token IDs used as traits in
			// game NFT schemas) don't round through float64's 2^53 limit.
			name: "large integer trait value retains full precision",
			body: `{"attributes":[{"trait_type":"BoundTokenID","value":18446744073709551615}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTokenMetadataJSON([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Spot-check string fields.
			if tt.name == "OpenSea-style happy path with attributes" {
				if got.Name != "Sword of Truth #1" || got.Description != "Legendary weapon" ||
					got.Image != "ipfs://QmFoo/1.png" || got.ExternalURL != "https://example.com/sword/1" {
					t.Errorf("string fields wrong: %+v", got)
				}
				if len(got.Attributes) != 3 {
					t.Fatalf("attributes count = %d, want 3", len(got.Attributes))
				}
				if got.Attributes[0].TraitType != "Damage" || got.Attributes[0].Value != "50" || got.Attributes[0].DisplayType != "number" {
					t.Errorf("attr[0] = %+v", got.Attributes[0])
				}
				if got.Attributes[1].TraitType != "Element" || got.Attributes[1].Value != "Fire" {
					t.Errorf("attr[1] = %+v", got.Attributes[1])
				}
				if got.Attributes[2].TraitType != "Cursed" || got.Attributes[2].Value != "true" {
					t.Errorf("attr[2] = %+v", got.Attributes[2])
				}
			}
			if tt.name == "non-object attrs entries skipped" {
				if len(got.Attributes) != 1 {
					t.Errorf("attributes count = %d, want 1 (the valid object)", len(got.Attributes))
				}
			}
			if tt.name == "float trait value preserves precision" {
				if len(got.Attributes) != 1 || got.Attributes[0].Value != "0.5" {
					t.Errorf("float attr = %+v", got.Attributes)
				}
			}
			if tt.name == "large integer trait value retains full precision" {
				// 18446744073709551615 = 2^64 - 1, well past 2^53.
				// Without UseNumber this would round to 1.8446744073709552e+19.
				if len(got.Attributes) != 1 || got.Attributes[0].Value != "18446744073709551615" {
					t.Errorf("large-int attr = %+v (want value=\"18446744073709551615\")", got.Attributes)
				}
			}
		})
	}
}

func TestStringifyAttrField(t *testing.T) {
	tests := []struct {
		in   interface{}
		want string
	}{
		{nil, ""},
		{"hello", "hello"},
		{"  trim me  ", "trim me"},
		{true, "true"},
		{false, "false"},
		{float64(42), "42"},
		{float64(0.5), "0.5"},
		{float64(0), "0"},
		{[]interface{}{1, 2}, ""},
		{map[string]interface{}{"k": "v"}, ""},
		// json.Number variants (the UseNumber path).
		{json.Number("42"), "42"},
		{json.Number("0.5"), "0.5"},
		// 2^64 - 1: lossless via json.Number, would lose precision via float64.
		{json.Number("18446744073709551615"), "18446744073709551615"},
	}
	for _, tt := range tests {
		if got := stringifyAttrField(tt.in); got != tt.want {
			t.Errorf("stringifyAttrField(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseMetadataJSONNullLiteral(t *testing.T) {
	// JSON literal `null` unmarshals into a nil map. The defensive nil
	// check in stringField keeps the parser from panicking; we just
	// return an empty doc + nil error.
	doc, err := parseMetadataJSON([]byte("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Name != "" || doc.Description != "" || doc.Image != "" || doc.ExternalURL != "" {
		t.Errorf("expected empty doc, got %+v", doc)
	}
}

func TestIsForbiddenIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		// Public, allowed
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"209.250.255.226", false}, // QRL Foundation public RPC
		{"46.62.169.114", false},   // Our testnet node
		{"2606:4700:4700::1111", false}, // Cloudflare DNS over IPv6

		// Loopback
		{"127.0.0.1", true},
		{"127.0.0.53", true},
		{"::1", true},

		// RFC1918 private
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},

		// Link-local
		{"169.254.169.254", true}, // AWS/GCP metadata!
		{"fe80::1", true},

		// CGNAT
		{"100.64.0.1", true},
		{"100.127.255.254", true},

		// Multicast / unspecified
		{"224.0.0.1", true},
		{"0.0.0.0", true},
		{"::", true},

		// IPv6 ULA
		{"fc00::1", true},
		{"fd12:3456::1", true},

		// IETF reserved / docs
		{"192.0.2.1", true},
		{"198.51.100.1", true},
		{"203.0.113.5", true},
		{"240.0.0.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("bad test fixture: cannot parse %q", tt.ip)
			}
			if got := isForbiddenIP(ip); got != tt.want {
				t.Errorf("isForbiddenIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// TestForbiddenIPRejectsNil documents the nil-input behaviour: refuse,
// don't crash, callers that fail to resolve a hostname must not dial.
func TestForbiddenIPRejectsNil(t *testing.T) {
	if !isForbiddenIP(nil) {
		t.Error("isForbiddenIP(nil) = false, want true (refuse-by-default)")
	}
}

func TestLooksLikeCID(t *testing.T) {
	cases := map[string]bool{
		"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG":             true,
		"bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi": true,
		"definitely-not-a-cid":                                       false,
		"Qm-too-short":                                               false,
		"":                                                           false,
		strings.Repeat("X", 60):                                       false,
	}
	for in, want := range cases {
		if got := looksLikeCID(in); got != want {
			t.Errorf("looksLikeCID(%q) = %v, want %v", in, got, want)
		}
	}
}
