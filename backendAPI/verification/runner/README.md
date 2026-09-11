# Hyperion compiler registry and sandbox

The contract verifier selects a pinned Hyperion compiler by `buildId`, compiles
standard JSON in a one-shot subprocess, and byte-matches the result against the
deployed runtime code. Enabled verification requires Linux, a fully static
x86-64 Hyperion compiler, and a fully static x86-64 NsJail launcher.

## Sealed execution trust chain

Registry initialization fails closed unless both executable artifacts have
absolute paths and exact SHA-256 pins. For each artifact, the backend:

1. Rejects symlinks, FIFO, device, socket, non-regular, non-executable, and
   group-writable or world-writable paths. Linux opens use `openat2` with
   `RESOLVE_NO_SYMLINKS`, `RESOLVE_NO_MAGICLINKS`, `O_NONBLOCK`, and a
   descriptor identity check.
2. Requires a Linux x86-64 ELF with no `PT_INTERP` program header and no
   `DT_NEEDED` dependency. Host loaders and shared-library trees are outside
   the execution identity and are therefore unsupported.
3. Copies the exact open descriptor into an executable `memfd`, calculates its
   SHA-256 during that copy, requires the configured digest, and seals the
   snapshot against writes, growth, shrinking, execute-mode changes, and
   further seal changes.

The backend also embeds [`nsjail.cfg`](./nsjail.cfg), verifies its exact
build-time SHA-256, copies it into a read-only non-executable `memfd`, and seals
it. The current policy digest is:

```text
cc2c6d14e943c9b4b9252e69c2fbf9a5a4313938cd561594a255746393baaa8c
```

Each compiler version probe and compile launches the sealed NsJail snapshot as
child fd 3. NsJail reads the sealed policy from child fd 5 and executes the
sealed Hyperion snapshot by fd from child fd 4. No trusted host pathname is
resolved after registry initialization. Replacing the configured NsJail,
Hyperion, or manifest pathname cannot change a running registry's retained
bytes.

Registry shutdown closes all retained descriptors. Initialization fails closed
outside Linux and when the kernel does not support race-free `openat2`,
executable sealed memfds, or the required NsJail setup. Explicit `MFD_EXEC`
supports hardened `vm.memfd_noexec=1` configurations that require executable
intent at memfd creation.

## Sandbox policy

The embedded policy creates new user, mount, PID, IPC, UTS, network, and cgroup
namespaces for every compiler invocation. It provides a 4 KiB empty read-only
tmpfs as `/`, mounts no host path, `/proc`, `/dev`, or `/sys`, disables
loopback, clears the environment, maps the child to uid and gid 65534, retains
no capabilities, and applies `no_new_privs`.

The following child limits are mandatory and fixed in the policy:

- address space: 2048 MiB
- CPU time: 30 seconds
- core size: 0 bytes
- regular-file output: 0 bytes
- open descriptors: 16
- processes: 1
- stack: 64 MiB
- locked memory, POSIX message queues, and real-time priority: 0

The Kafel seccomp policy is `DEFAULT KILL_PROCESS`. Its small allowlist is based
on direct traces of the pinned static Hyperion version and a representative
optimized contract compile. Read and write are restricted to standard streams,
file metadata and ioctl operations are restricted to descriptors 0 through 2,
executable memory mappings are denied, and `prlimit64` is restricted to querying
or lowering the current process's configured resource classes.

The allowlist excludes all socket and connect operations; `clone`, `clone3`,
`fork`, and `vfork`; `ptrace` and cross-process memory access; mount and
namespace mutation; BPF, perf, io_uring, keyring, userfaultfd, module, kexec,
reboot, and privilege-changing operations; filesystem opens and mutation; and
new memfd creation. `RLIMIT_NPROC=1` is defense in depth for the seccomp process
creation boundary.

NsJail aborts before compiler execution if it cannot create a required
namespace, mount the empty root, apply an rlimit, compile or install seccomp, or
otherwise complete containment. The Go parent retains its shared concurrency
semaphore, request and output caps, wall deadline, process-group kill,
parent-death signal, and bounded command wait. NsJail's 30-second limit remains
an inner ceiling when the Go wall timeout is configured higher.

## Configuring the launcher

Set these global values in addition to the compiler registry:

```bash
VERIFIER_SANDBOX_BIN=/absolute/path/to/nsjail-static
VERIFIER_SANDBOX_SHA256=<exact-64-character-sha256>
```

Both settings are mandatory. The digest should come from the exact deployment
artifact, not from a tag name or source commit alone.

[`build-nsjail-static.sh`](./build-nsjail-static.sh) builds from an already
initialized, clean checkout and refuses source commits other than NsJail 3.6 at
`f78475530b46d0186111a9096b30725f816b55fe` with Kafel at
`76d0f41bf3eb5c4008713d64b9767b461a9129a3`. It requires local static protobuf,
zlib, libnl-route, libnl, and pthread development artifacts. No generated binary
is tracked.

```bash
git clone --branch 3.6 --recurse-submodules https://github.com/google/nsjail.git /absolute/build/nsjail
git -C /absolute/build/nsjail rev-parse HEAD
git -C /absolute/build/nsjail/kafel rev-parse HEAD
./build-nsjail-static.sh /absolute/build/nsjail /absolute/output/nsjail-static
file /absolute/output/nsjail-static
readelf -l /absolute/output/nsjail-static
readelf -d /absolute/output/nsjail-static
sha256sum /absolute/output/nsjail-static
```

Set `NSJAIL_EXPECTED_SHA256` when reproducing an artifact whose exact binary
digest is already approved. The recipe never embeds a workstation-specific
binary digest.

## Configuring compiler builds

Set `HYPC_COMPILERS` to either an inline JSON array or an absolute path to a JSON
file. A file-backed manifest also requires `HYPC_COMPILERS_SHA256`, the exact
SHA-256 of the manifest bytes. The manifest must be regular, contain no symlink
in its resolved path, and must not be group-writable or world-writable. JSON
parsing rejects comments, trailing content, trailing commas, unknown fields,
and duplicate object members.

See [`compilers.example.json`](./compilers.example.json). The fields are:

- `kind`: required. Enabled entries must be `native`.
- `buildId`: required exact version from the compiler's `Version:` output.
- `bin`: required absolute path to a fully static Linux x86-64 compiler.
- `sha256`: required 64-character hexadecimal artifact digest.
- `disabled`: optional historical identity that cannot be selected.
- `default`: exactly one enabled entry must set this to `true`.

`runner` and `nodeBin` are rejected on enabled native entries. Enabled npm
entries fail closed because their complete JS, WASM, Node, and loader runtime
identity is unavailable. A non-default historical native entry that fails its
artifact or version probe is skipped. A failed explicit default makes registry
initialization fail, so a historical build is never promoted implicitly.

The current local Q128 compiler identity is:

```text
Version: 0.2.0-develop.2026.8.27+commit.6f862206.mod.Linux.g++
SHA-256: ac24ccbb53fa6200dc6ca9a3bb87aac5dbb8dd7a23a3f91c3a7563f6275882dd
Hyperion source commit: 6f862206ff34bce56098cc23b4b3f575e0ea3c5f
```

The `.mod` suffix and binary digest identify the audit-updated local static
artifact. The previous dynamic artifact is unsupported because it has a host
loader and shared-library dependency tree. Historical identities remain
disabled until matching static artifacts are available.

`compilers.json` is ignored so absolute local paths remain outside tracked
configuration. Build Hyperion in a separate static build directory, copy the
example manifest, replace its placeholders, set its mode to `0600`, and verify
both artifacts before backend startup:

```bash
cmake -S ../../../../hyperion -B ../../../../hyperion/build-static -DHYPC_LINK_STATIC=ON
cmake --build ../../../../hyperion/build-static --target hypc
realpath ../../../../hyperion/build-static/hypc/hypc
../../../../hyperion/build-static/hypc/hypc --version
file ../../../../hyperion/build-static/hypc/hypc
readelf -l ../../../../hyperion/build-static/hypc/hypc
readelf -d ../../../../hyperion/build-static/hypc/hypc
sha256sum ../../../../hyperion/build-static/hypc/hypc
chmod 0600 compilers.json
sha256sum compilers.json
```

The `readelf` checks must show no `INTERP` program header and no `NEEDED`
dynamic tag. Startup repeats those checks from the securely opened descriptors.

## Legacy compiler environment

When `HYPC_COMPILERS` is unset, `HYPC_BUILD_ID`, `HYPC_BIN`, and `HYPC_SHA256`
can synthesize one native default entry:

```bash
HYPC_BUILD_ID=0.2.0-develop.2026.8.27+commit.6f862206.mod.Linux.g++
HYPC_BIN=/absolute/path/to/hypc-6f862206-static
HYPC_SHA256=ac24ccbb53fa6200dc6ca9a3bb87aac5dbb8dd7a23a3f91c3a7563f6275882dd
```

`HYPC_RUNNER` and `HYPC_NODE_BIN` do not enter the native execution chain. A
runner-only legacy configuration is treated as npm and fails closed.

## Provenance v2

`GET /contract/compiler-info` exposes a canonical provenance record for every
selectable build. Successful verification persists the same record in the job
payload, job result, and contract document.

Schema `qrl.contract-compiler-provenance.v2` binds the build id and these exact,
ordered components:

1. `hypc`: the sealed compiler artifact SHA-256
2. `nsjail`: the sealed launcher artifact SHA-256
3. `policy`: the exact embedded NsJail config SHA-256

The execution digest uses unsigned 64-bit big-endian length prefixes for every
variable field and component count, which makes the canonical byte sequence
unambiguous. Schema v1 records lack sandbox identity and classify as
`invalid-recorded`. An absent provenance field on an older record remains
`legacy-unrecorded`; the backend never infers old provenance from the current
registry.

## Shared Go resource limits

```bash
VERIFIER_MAX_CONCURRENCY=2
VERIFIER_COMPILE_TIMEOUT=30s
VERIFIER_SOURCE_MAX_BYTES=262144
VERIFIER_COMPILER_MAX_STDOUT_BYTES=16777216
VERIFIER_COMPILER_MAX_STDERR_BYTES=262144
```

These limits apply across every enabled compiler build. Reaching either output
cap cancels the compile, kills the Linux process group, and returns a distinct
stdout or stderr exhaustion error.

## Local real-sandbox tests

Regular tests build test-only static launchers and require no installed NsJail.
The hostile and representative Hyperion probes are opt-in so CI can provide its
own pinned artifacts:

```bash
VERIFIER_REAL_SANDBOX_TEST=1 \
VERIFIER_SANDBOX_BIN=/absolute/path/to/nsjail-static \
VERIFIER_SANDBOX_SHA256=<nsjail-sha256> \
VERIFIER_TEST_HYPC_BIN=/absolute/path/to/hypc-static \
VERIFIER_TEST_HYPC_SHA256=<hypc-sha256> \
VERIFIER_TEST_HYPC_BUILD_ID='<exact-Version-field>' \
go test -count=1 -v ./verification -run 'TestReal(Sandbox|Static)'
```

The probes validate the empty root and environment, hostname, mandatory
rlimits, host read and write denial, TCP and Unix socket denial, process
creation denial, invalid-policy startup failure, a representative optimized
Hyperion compile, and byte-for-byte equality with direct compiler output.

## Deployment gate

Production must provision the reviewed static NsJail and Hyperion artifacts,
set their exact digests, and run a real-sandbox smoke test on the deployment
kernel. Kernel policy must permit the required unprivileged user, mount, PID,
IPC, UTS, network, and cgroup namespaces. The verifier intentionally stays
unavailable if any artifact, namespace, memfd, mount, rlimit, or seccomp setup
cannot be established.

`hypc-native.sh` and `hypc-runner.js` remain manual compatibility utilities.
The backend does not execute either file in the enabled native trust chain.
