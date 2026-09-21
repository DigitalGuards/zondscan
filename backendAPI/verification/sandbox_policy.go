package verification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const sandboxPolicyExpectedSHA256 = "cc2c6d14e943c9b4b9252e69c2fbf9a5a4313938cd561594a255746393baaa8c"

//go:embed runner/nsjail.cfg
var sandboxPolicy []byte

func sandboxPolicySHA256() (string, error) {
	digest := sha256.Sum256(sandboxPolicy)
	actual := hex.EncodeToString(digest[:])
	if actual != sandboxPolicyExpectedSHA256 {
		return "", fmt.Errorf(
			"embedded NsJail policy SHA-256 mismatch: want %s, got %s",
			sandboxPolicyExpectedSHA256,
			actual,
		)
	}
	return actual, nil
}
