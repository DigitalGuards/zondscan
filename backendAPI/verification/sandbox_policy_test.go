package verification

import (
	"reflect"
	"strings"
	"testing"
)

func TestSandboxPolicyDigestAndRequiredIsolation(t *testing.T) {
	digest, err := sandboxPolicySHA256()
	if err != nil {
		t.Fatalf("sandboxPolicySHA256: %v", err)
	}
	if digest != sandboxPolicyExpectedSHA256 {
		t.Fatalf("policy digest = %q, want %q", digest, sandboxPolicyExpectedSHA256)
	}

	policy := string(sandboxPolicy)
	required := []string{
		`mode: ONCE`,
		`keep_env: false`,
		`keep_caps: false`,
		`disable_no_new_privs: false`,
		`rlimit_as: 2048`,
		`rlimit_as_type: VALUE`,
		`rlimit_core: 0`,
		`rlimit_core_type: VALUE`,
		`rlimit_cpu: 30`,
		`rlimit_cpu_type: VALUE`,
		`rlimit_fsize: 0`,
		`rlimit_fsize_type: VALUE`,
		`rlimit_nofile: 16`,
		`rlimit_nofile_type: VALUE`,
		`rlimit_nproc: 1`,
		`rlimit_nproc_type: VALUE`,
		`clone_newnet: true`,
		`clone_newuser: true`,
		`clone_newns: true`,
		`clone_newpid: true`,
		`clone_newipc: true`,
		`clone_newuts: true`,
		`clone_newcgroup: true`,
		`use_cgroupv2: false`,
		`detect_cgroupv2: false`,
		`mount_proc: false`,
		`dst: "/"`,
		`fstype: "tmpfs"`,
		`options: "size=4096,mode=0555"`,
		`rw: false`,
		`is_dir: true`,
		`mandatory: true`,
		`nosuid: true`,
		`nodev: true`,
		`noexec: true`,
		`iface_no_lo: true`,
		`path: "/proc/self/fd/4"`,
		`exec_fd: true`,
		`DEFAULT KILL_PROCESS`,
	}
	for _, fragment := range required {
		if !strings.Contains(policy, fragment) {
			t.Errorf("sandbox policy is missing %q", fragment)
		}
	}
	for _, forbidden := range []string{
		`keep_env: true`,
		`keep_caps: true`,
		`disable_no_new_privs: true`,
		`mount_proc: true`,
		`pass_fd:`,
		`envar:`,
		`src:`,
		`DEFAULT ALLOW`,
	} {
		if strings.Contains(policy, forbidden) {
			t.Errorf("sandbox policy contains forbidden setting %q", forbidden)
		}
	}
}

func TestSandboxSeccompAllowlistIsExactAndExcludesDangerousSyscalls(t *testing.T) {
	const prefix = `seccomp_string: "ALLOW { `
	const suffix = ` }\nDEFAULT KILL_PROCESS"`
	policy := string(sandboxPolicy)
	start := strings.Index(policy, prefix)
	if start < 0 {
		t.Fatal("sandbox policy has no Kafel ALLOW block")
	}
	start += len(prefix)
	end := strings.Index(policy[start:], suffix)
	if end < 0 {
		t.Fatal("sandbox policy has no DEFAULT KILL_PROCESS suffix")
	}
	entries := strings.Split(policy[start:start+end], ",")
	for i := range entries {
		entries[i] = strings.TrimSpace(entries[i])
	}
	entryNames := make([]string, len(entries))
	for i, entry := range entries {
		entryNames[i] = strings.Fields(entry)[0]
	}
	want := []string{
		"read", "write", "newfstat", "ioctl", "brk", "mmap", "mprotect",
		"munmap", "madvise", "arch_prctl", "set_tid_address", "set_robust_list",
		"rseq", "prlimit64", "readlinkat", "getrandom", "newuname", "futex",
		"exit", "exit_group", "execveat",
	}
	if !reflect.DeepEqual(entryNames, want) {
		t.Fatalf("seccomp allowlist = %#v, want %#v", entryNames, want)
	}
	for _, filter := range []string{
		`read { fd == 0 }`,
		`write { fd == 1 || fd == 2 }`,
		`newfstat { fd <= 2 }`,
		`ioctl { fd <= 2 }`,
		`mmap { (prot & 0x4) == 0 }`,
		`mprotect { (prot & 0x4) == 0 }`,
		`prlimit64 { pid == 0 && (resource == 0 || resource == 1 || resource == 3 || resource == 4 || resource == 6 || resource == 7 || resource == 9) }`,
	} {
		if !strings.Contains(policy, filter) {
			t.Errorf("seccomp policy is missing argument filter %q", filter)
		}
	}
	allowed := make(map[string]struct{}, len(entryNames))
	for _, entry := range entryNames {
		allowed[entry] = struct{}{}
	}
	for _, syscall := range []string{
		"open", "openat", "openat2", "creat", "unlink", "unlinkat", "rename",
		"renameat", "renameat2", "mkdir", "mkdirat", "rmdir", "link", "linkat",
		"symlink", "symlinkat", "chmod", "fchmod", "fchmodat", "chown", "fchown",
		"fchownat", "truncate", "ftruncate",
		"socket", "socketpair", "connect", "bind", "listen", "accept", "accept4",
		"clone", "clone3", "fork", "vfork", "ptrace", "process_vm_readv",
		"process_vm_writev", "mount", "umount2", "fsopen", "fsmount", "move_mount",
		"open_tree", "pivot_root", "chroot", "setns", "unshare", "bpf", "keyctl", "add_key", "request_key",
		"perf_event_open", "userfaultfd", "init_module", "finit_module", "delete_module",
		"kexec_load", "kexec_file_load", "memfd_create", "execve",
	} {
		if _, ok := allowed[syscall]; ok {
			t.Errorf("dangerous syscall %q is present in the allowlist", syscall)
		}
	}
}
