package sourcebundle

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strings"
)

// Version identifies the deterministic source framing used by the verifier
// and AI explainer. Changing file labels, ordering, or separators requires a
// new version so cached explanations cannot cross formats.
const Version = "qrl.verified-source-bundle.v1"

const digestPrefix = Version + ":sha256:"

// Render returns the exact source frame consumed by the AI explainer. The
// primary source is first and imports are ordered by canonical filename.
func Render(contractName, sourceCode string, imports map[string]string) string {
	primaryFilename := "primary.hyp"
	if contractName != "" {
		primaryFilename = contractName + ".hyp"
	}

	var b strings.Builder
	writeFile(&b, primaryFilename, "primary", sourceCode)

	filenames := make([]string, 0, len(imports))
	for filename := range imports {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		writeFile(&b, filename, "import", imports[filename])
	}
	return b.String()
}

// Digest returns a versioned SHA-256 identity for the exact source bundle.
// Every role, filename, and content field is length-prefixed so source text
// cannot impersonate a file boundary and trailing newlines remain significant.
func Digest(contractName, sourceCode string, imports map[string]string) string {
	primaryFilename := "primary.hyp"
	if contractName != "" {
		primaryFilename = contractName + ".hyp"
	}

	h := sha256.New()
	_, _ = h.Write([]byte(Version))
	_, _ = h.Write([]byte{0})
	writeUint64(h, uint64(1+len(imports)))
	writeDigestFile(h, "primary", primaryFilename, sourceCode)

	filenames := make([]string, 0, len(imports))
	for filename := range imports {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		writeDigestFile(h, "import", filename, imports[filename])
	}
	return digestPrefix + hex.EncodeToString(h.Sum(nil))
}

func writeDigestFile(h hash.Hash, role, filename, source string) {
	writeDigestField(h, role)
	writeDigestField(h, filename)
	writeDigestField(h, source)
}

func writeDigestField(h hash.Hash, value string) {
	writeUint64(h, uint64(len(value)))
	_, _ = h.Write([]byte(value))
}

func writeUint64(h hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = h.Write(encoded[:])
}

func writeFile(b *strings.Builder, filename, role, source string) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "// File: %q (%s)\n", filename, role)
	b.WriteString(source)
	if !strings.HasSuffix(source, "\n") {
		b.WriteString("\n")
	}
}
