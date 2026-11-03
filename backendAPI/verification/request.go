package verification

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"backendAPI/models"
)

const (
	defaultOptimizerRuns = 200

	// maxSourceFiles caps the primary source plus every import in one
	// request. The marshalled standard-JSON payload is also capped by
	// VERIFIER_SOURCE_MAX_BYTES, so this limit only bounds the number of
	// source units the path checks below have to compare.
	maxSourceFiles = 128
	// maxSourcePathBytes and maxSourcePathSegments bound one source path.
	maxSourcePathBytes    = 256
	maxSourcePathSegments = 32
)

var hyperionIdentifierPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// CanonicalizeVerifyRequest resolves every request value that can change the
// compiler input before the request is persisted or handed to the async
// runner. It is idempotent. The returned imports map is a fresh copy with
// clean slash-separated relative paths, and PrimarySourcePath is empty when
// it names the default <contractName>.hyp layout.
//
// Source paths are source unit names in the standard-JSON `sources` map,
// which is the compiler's entire virtual file system inside the sandbox.
// No path ever reaches the host file system, so the checks here guard the
// shape of that virtual tree: relative, inside its root, unambiguous, and
// limited in size.
func CanonicalizeVerifyRequest(req VerifyRequest) (VerifyRequest, error) {
	if !hyperionIdentifierPattern.MatchString(req.ContractName) {
		return VerifyRequest{}, fmt.Errorf(
			"contractName must be a Hyperion identifier matching %s",
			hyperionIdentifierPattern.String(),
		)
	}

	primary := models.DefaultPrimarySourcePath(req.ContractName)
	if req.PrimarySourcePath != "" {
		cleanPrimary, err := canonicalSourcePath("primarySourcePath", req.PrimarySourcePath)
		if err != nil {
			return VerifyRequest{}, err
		}
		if path.Ext(cleanPrimary) != ".hyp" || path.Base(cleanPrimary) == ".hyp" {
			return VerifyRequest{}, fmt.Errorf(
				"primarySourcePath %q must name a .hyp file",
				req.PrimarySourcePath,
			)
		}
		primary = cleanPrimary
	}
	req.PrimarySourcePath = models.CanonicalPrimarySourcePath(req.ContractName, primary)

	imports, err := canonicalizeImports(models.DefaultPrimarySourcePath(req.ContractName), primary, req.Imports)
	if err != nil {
		return VerifyRequest{}, err
	}
	req.Imports = imports
	if req.OptimizerEnabled && req.OptimizerRuns <= 0 {
		req.OptimizerRuns = defaultOptimizerRuns
	}
	return req, nil
}

// effectivePrimarySourcePath is the source unit name the primary source is
// compiled under and the unit the requested contract is selected from.
func effectivePrimarySourcePath(req VerifyRequest) string {
	return models.EffectivePrimarySourcePath(req.ContractName, req.PrimarySourcePath)
}

func canonicalizeImports(defaultPrimary, primary string, imports map[string]string) (map[string]string, error) {
	if len(imports) == 0 {
		return nil, nil
	}
	if len(imports)+1 > maxSourceFiles {
		return nil, fmt.Errorf(
			"too many source files: %d imports plus the primary source exceed the limit of %d",
			len(imports),
			maxSourceFiles,
		)
	}

	// Walk the raw paths in a fixed order so a request with several
	// problems always reports the same one.
	rawPaths := make([]string, 0, len(imports))
	for rawPath := range imports {
		rawPaths = append(rawPaths, rawPath)
	}
	sort.Strings(rawPaths)

	canonical := make(map[string]string, len(imports))
	seen := make(map[string]string, len(imports))
	seenFolded := map[string]string{strings.ToLower(primary): primary}
	for _, rawPath := range rawPaths {
		cleanPath, err := canonicalSourcePath("import path", rawPath)
		if err != nil {
			return nil, err
		}
		if cleanPath == primary {
			return nil, fmt.Errorf("import path %q collides with primary source %q", rawPath, primary)
		}
		// The verified source bundle the AI explainer reads always labels
		// the primary source <contractName>.hyp, wherever it was compiled.
		// With a nested primary that label stays reserved, so a root import
		// under the same name cannot pose as a second copy of the primary.
		if primary != defaultPrimary && strings.EqualFold(cleanPath, defaultPrimary) {
			return nil, fmt.Errorf(
				"import path %q uses the name %q, which the verified source bundle reserves for the primary source %q",
				rawPath,
				defaultPrimary,
				primary,
			)
		}
		if previous, exists := seen[cleanPath]; exists {
			return nil, fmt.Errorf(
				"import paths %q and %q normalize to the same source %q",
				previous,
				rawPath,
				cleanPath,
			)
		}
		// Paths that differ only by letter case name one file on the
		// case-insensitive file systems macOS and Windows use by default, so
		// such a tree cannot be checked out there, and the two names read as
		// the same file to a reviewer. They stay rejected even though a Linux
		// checkout can hold both.
		folded := strings.ToLower(cleanPath)
		if previous, exists := seenFolded[folded]; exists {
			return nil, fmt.Errorf(
				"source paths %q and %q differ only by letter case",
				previous,
				cleanPath,
			)
		}
		seen[cleanPath] = rawPath
		seenFolded[folded] = cleanPath
		canonical[cleanPath] = imports[rawPath]
	}

	if err := rejectFileDirectoryConflicts(seenFolded); err != nil {
		return nil, err
	}
	return canonical, nil
}

// rejectFileDirectoryConflicts refuses a tree in which one path is both a
// file and a directory, such as "lib.hyp" next to "lib.hyp/Math.hyp". A real
// checkout cannot hold both, so such a request can only be an alias trick.
// The keys of foldedPaths are lower-cased so the check also covers
// case-insensitive checkouts.
func rejectFileDirectoryConflicts(foldedPaths map[string]string) error {
	folded := make([]string, 0, len(foldedPaths))
	for key := range foldedPaths {
		folded = append(folded, key)
	}
	sort.Strings(folded)
	for _, key := range folded {
		for dir := path.Dir(key); dir != "."; dir = path.Dir(dir) {
			if file, exists := foldedPaths[dir]; exists {
				return fmt.Errorf(
					"source path %q is used as a file and as a directory of %q",
					file,
					foldedPaths[key],
				)
			}
		}
	}
	return nil
}

// canonicalSourcePath validates one repo-relative source path and returns its
// clean form. The character set is deliberately narrow: ASCII letters,
// digits, spaces, ".", "_", "-", "@", "+" and "~" separated by "/". Spaces
// stay allowed because the single-file imports flow always accepted names
// such as "./My Token.hyp". The set excludes NUL and control characters,
// backslashes, URL schemes, and Unicode look-alikes or direction overrides
// that could disguise a path on the verified-source page.
func canonicalSourcePath(field, rawPath string) (string, error) {
	if rawPath == "" {
		return "", fmt.Errorf("%s is empty", field)
	}
	if len(rawPath) > maxSourcePathBytes {
		return "", fmt.Errorf("%s %q is longer than %d bytes", field, truncateForError(rawPath), maxSourcePathBytes)
	}
	for i := 0; i < len(rawPath); i++ {
		c := rawPath[i]
		if c == '\\' {
			return "", fmt.Errorf("%s %q must use forward slashes", field, rawPath)
		}
		if !isSourcePathByte(c) {
			return "", fmt.Errorf(
				"%s %q contains an unsupported character; use ASCII letters, digits, spaces, and . _ - @ + ~ separated by /",
				field,
				rawPath,
			)
		}
	}
	if path.IsAbs(rawPath) {
		return "", fmt.Errorf("%s %q must be relative", field, rawPath)
	}

	cleanPath := path.Clean(rawPath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("%s %q escapes the source root", field, rawPath)
	}
	if segments := strings.Count(cleanPath, "/") + 1; segments > maxSourcePathSegments {
		return "", fmt.Errorf(
			"%s %q has %d path segments, more than the limit of %d",
			field,
			rawPath,
			segments,
			maxSourcePathSegments,
		)
	}
	return cleanPath, nil
}

func isSourcePathByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '/', '.', '_', '-', '@', '+', '~', ' ':
		return true
	}
	return false
}

func truncateForError(value string) string {
	const keep = 64
	if len(value) <= keep {
		return value
	}
	return value[:keep] + "..."
}
