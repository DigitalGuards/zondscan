package configs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type syncerLeaseIdentity struct {
	machineID string
	bootID    string
}

var (
	syncerLeaseMachineIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	syncerLeaseBootIDPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	syncerLeaseOwnerPattern     = regexp.MustCompile(`^(?:[a-z0-9]+-)*pid-[1-9][0-9]*-[0-9a-f]{32};machine=([0-9a-f]{32});boot=([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`)
)

func (identity syncerLeaseIdentity) known() bool {
	return syncerLeaseMachineIDPattern.MatchString(identity.machineID) &&
		syncerLeaseBootIDPattern.MatchString(identity.bootID)
}

// readSyncerLeaseIdentity requires both Linux identities. Missing or malformed
// values leave recovery under operator control.
func readSyncerLeaseIdentity(readFile func(string) ([]byte, error)) syncerLeaseIdentity {
	machine, err := readFile("/etc/machine-id")
	if err != nil {
		return syncerLeaseIdentity{}
	}
	boot, err := readFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return syncerLeaseIdentity{}
	}
	machineID := strings.TrimSpace(string(machine))
	bootID := strings.TrimSpace(string(boot))
	if !syncerLeaseMachineIDPattern.MatchString(machineID) || !syncerLeaseBootIDPattern.MatchString(bootID) {
		return syncerLeaseIdentity{}
	}
	// Keep the first 128 bits of SHA-256 so the raw machine ID stays private.
	digest := sha256.Sum256([]byte(machineID))
	return syncerLeaseIdentity{machineID: hex.EncodeToString(digest[:16]), bootID: bootID}
}

func formatSyncerLeaseOwner(pid int, random []byte, identity syncerLeaseIdentity) string {
	owner := fmt.Sprintf("pid-%d-%s", pid, hex.EncodeToString(random))
	if identity.known() {
		owner += ";machine=" + identity.machineID + ";boot=" + identity.bootID
	}
	return owner
}

// parseSyncerLeaseOwner accepts the optional tool prefixes used by maintenance
// writers. Legacy owners and malformed identities have unknown identity.
func parseSyncerLeaseOwner(owner string) syncerLeaseIdentity {
	parts := syncerLeaseOwnerPattern.FindStringSubmatch(owner)
	if parts == nil {
		return syncerLeaseIdentity{}
	}
	return syncerLeaseIdentity{machineID: parts[1], bootID: parts[2]}
}

func NewSyncerLeaseOwner() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate syncer lease owner: %w", err)
	}
	return formatSyncerLeaseOwner(os.Getpid(), random, readSyncerLeaseIdentity(os.ReadFile)), nil
}
