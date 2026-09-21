package explainauth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeAuthorizationRequestStrictShape(t *testing.T) {
	request := dummyAuthorizationRequest()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeAuthorizationRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeAuthorizationRequest: %v", err)
	}
	if decoded.ChallengeID != request.ChallengeID || decoded.Proof.Signer != request.Proof.Signer {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDecodeAuthorizationRequestRejectsAmbiguousOrMalformedJSON(t *testing.T) {
	request := dummyAuthorizationRequest()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(encoded)
	tests := map[string]string{
		"duplicate root key":     strings.Replace(valid, `{"challengeId":`, `{"challengeId":"`+request.ChallengeID+`","challengeId":`, 1),
		"duplicate proof key":    strings.Replace(valid, `"signature":`, `"signature":"`+request.Proof.Signature+`","signature":`, 1),
		"case alias root key":    strings.Replace(valid, `{"challengeId":`, `{"ChallengeID":"`+request.ChallengeID+`","challengeId":`, 1),
		"case alias proof key":   strings.Replace(valid, `"signature":`, `"Signature":"`+request.Proof.Signature+`","signature":`, 1),
		"case changed root key":  strings.Replace(valid, `"challengeId":`, `"ChallengeID":`, 1),
		"case changed proof key": strings.Replace(valid, `"schemeVersion":`, `"SchemeVersion":`, 1),
		"unknown root key":       strings.TrimSuffix(valid, "}") + `,"extra":true}`,
		"unknown proof key":      strings.Replace(valid, `"schemeVersion":`, `"extra":true,"schemeVersion":`, 1),
		"trailing value":         valid + `{}`,
		"short signature":        strings.Replace(valid, request.Proof.Signature, "0x00", 1),
		"uppercase challenge":    strings.Replace(valid, request.ChallengeID, strings.ToUpper(request.ChallengeID), 1),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeAuthorizationRequest([]byte(body)); err == nil {
				t.Fatal("DecodeAuthorizationRequest succeeded")
			}
		})
	}
}

func dummyAuthorizationRequest() AuthorizationRequest {
	return AuthorizationRequest{
		ChallengeID: strings.Repeat("a", nonceBytes*2),
		Proof: SignedMessageProof{
			Signature:     "0x" + strings.Repeat("11", 4627),
			PublicKey:     "0x" + strings.Repeat("22", 2592),
			Descriptor:    "0x010000",
			Signer:        "Q" + strings.Repeat("1", 128),
			Digest:        "0x" + strings.Repeat("33", digestBytes),
			SchemeVersion: SchemeVersion,
		},
	}
}
