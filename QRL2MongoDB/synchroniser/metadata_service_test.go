package synchroniser

import (
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
