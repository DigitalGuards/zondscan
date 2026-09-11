package verification

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

const defaultOptimizerRuns = 200

var hyperionIdentifierPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// CanonicalizeVerifyRequest resolves every request value that can change the
// compiler input before the request is persisted or handed to the async
// runner. The returned imports map is a fresh copy with clean slash-separated
// relative paths.
func CanonicalizeVerifyRequest(req VerifyRequest) (VerifyRequest, error) {
	if !hyperionIdentifierPattern.MatchString(req.ContractName) {
		return VerifyRequest{}, fmt.Errorf(
			"contractName must be a Hyperion identifier matching %s",
			hyperionIdentifierPattern.String(),
		)
	}

	imports, err := canonicalizeImports(req.ContractName, req.Imports)
	if err != nil {
		return VerifyRequest{}, err
	}
	req.Imports = imports
	if req.OptimizerEnabled && req.OptimizerRuns <= 0 {
		req.OptimizerRuns = defaultOptimizerRuns
	}
	return req, nil
}

func canonicalizeImports(contractName string, imports map[string]string) (map[string]string, error) {
	if len(imports) == 0 {
		return nil, nil
	}

	primary := primarySourcePath(contractName)
	canonical := make(map[string]string, len(imports))
	seen := make(map[string]string, len(imports))
	for rawPath, content := range imports {
		cleanPath, err := canonicalImportPath(rawPath)
		if err != nil {
			return nil, err
		}
		if cleanPath == primary {
			return nil, fmt.Errorf("import path %q collides with primary source %q", rawPath, primary)
		}
		if previous, exists := seen[cleanPath]; exists {
			return nil, fmt.Errorf(
				"import paths %q and %q normalize to the same source %q",
				previous,
				rawPath,
				cleanPath,
			)
		}
		seen[cleanPath] = rawPath
		canonical[cleanPath] = content
	}
	return canonical, nil
}

func canonicalImportPath(rawPath string) (string, error) {
	if rawPath == "" || strings.ContainsRune(rawPath, '\x00') {
		return "", fmt.Errorf("import path %q is empty or contains NUL", rawPath)
	}
	if strings.Contains(rawPath, `\`) {
		return "", fmt.Errorf("import path %q must use forward slashes", rawPath)
	}
	if path.IsAbs(rawPath) {
		return "", fmt.Errorf("import path %q must be relative", rawPath)
	}

	cleanPath := path.Clean(rawPath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("import path %q escapes the source root", rawPath)
	}
	return cleanPath, nil
}

func primarySourcePath(contractName string) string {
	return contractName + ".hyp"
}
