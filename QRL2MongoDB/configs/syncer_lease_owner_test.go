package configs

import (
	"errors"
	"strings"
	"testing"
)

const (
	testSyncerMachineID = "11111111111111111111111111111111"
	testSyncerBootID    = "11111111-2222-4333-8444-555555555555"
	testSyncerOldBootID = "00000000-1111-4222-8333-444444444444"
	testSyncerOldOwner  = "pid-42-30313233343536373839616263646566"
)

func TestFormatSyncerLeaseOwner(t *testing.T) {
	tests := []struct {
		name     string
		identity syncerLeaseIdentity
		want     string
	}{
		{
			name:     "complete identity",
			identity: syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerBootID},
			want:     testSyncerOldOwner + ";machine=" + testSyncerMachineID + ";boot=" + testSyncerBootID,
		},
		{name: "unknown identity", want: testSyncerOldOwner},
		{name: "missing machine", identity: syncerLeaseIdentity{bootID: testSyncerBootID}, want: testSyncerOldOwner},
		{name: "missing boot", identity: syncerLeaseIdentity{machineID: testSyncerMachineID}, want: testSyncerOldOwner},
		{
			name:     "malformed identity",
			identity: syncerLeaseIdentity{machineID: "invalid", bootID: testSyncerBootID},
			want:     testSyncerOldOwner,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatSyncerLeaseOwner(42, []byte("0123456789abcdef"), test.identity)
			if got != test.want {
				t.Fatalf("owner = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseSyncerLeaseOwner(t *testing.T) {
	identity := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerBootID}
	owner := testSyncerOldOwner + ";machine=" + testSyncerMachineID + ";boot=" + testSyncerBootID
	tests := []struct {
		name  string
		owner string
		want  syncerLeaseIdentity
	}{
		{name: "new owner", owner: owner, want: identity},
		{name: "maintenance prefix", owner: "backfill-internal-" + owner, want: identity},
		{name: "old owner", owner: testSyncerOldOwner},
		{name: "unknown owner", owner: "integration-owner-a"},
		{name: "empty owner"},
		{name: "missing boot", owner: testSyncerOldOwner + ";machine=" + testSyncerMachineID},
		{name: "missing machine", owner: testSyncerOldOwner + ";boot=" + testSyncerBootID},
		{name: "invalid machine", owner: strings.Replace(owner, testSyncerMachineID, "invalid", 1)},
		{name: "short machine", owner: strings.Replace(owner, testSyncerMachineID, testSyncerMachineID[:16], 1)},
		{name: "invalid boot", owner: strings.Replace(owner, testSyncerBootID, "invalid", 1)},
		{name: "missing process", owner: strings.TrimPrefix(owner, testSyncerOldOwner)},
		{name: "malformed process", owner: strings.Replace(owner, "pid-42-", "pid-invalid-", 1)},
		{name: "extra field", owner: owner + ";extra=field"},
		{name: "duplicate machine", owner: owner + ";machine=" + testSyncerMachineID},
		{name: "duplicate boot", owner: owner + ";boot=" + testSyncerBootID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseSyncerLeaseOwner(test.owner); got != test.want {
				t.Fatalf("identity = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestReadSyncerLeaseIdentity(t *testing.T) {
	const machineID = "0123456789abcdef0123456789abcdef"
	want := syncerLeaseIdentity{machineID: "3eb1bd439947eb762998e566ccc2e099", bootID: testSyncerBootID}
	tests := []struct {
		name       string
		machine    string
		boot       string
		failedPath string
		want       syncerLeaseIdentity
	}{
		{name: "hash and trim", machine: machineID + "\n", boot: testSyncerBootID + "\n", want: want},
		{name: "machine read failure", machine: machineID, boot: testSyncerBootID, failedPath: "/etc/machine-id"},
		{name: "boot read failure", machine: machineID, boot: testSyncerBootID, failedPath: "/proc/sys/kernel/random/boot_id"},
		{name: "empty machine", boot: testSyncerBootID},
		{name: "empty boot", machine: machineID},
		{name: "malformed machine", machine: "uninitialized", boot: testSyncerBootID},
		{name: "malformed boot", machine: machineID, boot: "invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := readSyncerLeaseIdentity(func(path string) ([]byte, error) {
				if path == test.failedPath {
					return nil, errors.New("identity unavailable")
				}
				switch path {
				case "/etc/machine-id":
					return []byte(test.machine), nil
				case "/proc/sys/kernel/random/boot_id":
					return []byte(test.boot), nil
				default:
					t.Fatalf("unexpected identity path %q", path)
					return nil, nil
				}
			})
			if got != test.want {
				t.Fatalf("identity = %+v, want %+v", got, test.want)
			}
			if test.want == (syncerLeaseIdentity{}) {
				owner := formatSyncerLeaseOwner(42, []byte("0123456789abcdef"), got)
				if owner != testSyncerOldOwner || canRecoverSyncerLeaseAfterReboot(SyncerLease{
					Owner: formatSyncerLeaseOwner(1, []byte("0123456789abcdef"), want), Generation: 1,
				}, true, got) {
					t.Fatal("unavailable identity must preserve the legacy owner and manual recovery")
				}
			}
		})
	}
}
