//go:build linux

package verification

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const maxNativeExecutableBytes int64 = 256 << 20

// snapshotExecutable copies one exact compiler artifact into a sealed memfd.
// The source descriptor is opened without following a final symlink, and the
// hash is computed over the same bytes written to the retained executable fd.
func snapshotExecutable(sourcePath, expectedSHA256 string) (*os.File, error) {
	return snapshotStaticExecutable(
		sourcePath,
		expectedSHA256,
		"native compiler",
		"zondscan-hypc",
	)
}

func snapshotSandboxLauncher(sourcePath, expectedSHA256 string) (*os.File, error) {
	return snapshotStaticExecutable(
		sourcePath,
		expectedSHA256,
		"NsJail sandbox launcher",
		"zondscan-nsjail",
	)
}

func snapshotStaticExecutable(
	sourcePath string,
	expectedSHA256 string,
	kind string,
	memfdName string,
) (*os.File, error) {
	source, info, err := openSecureRegularFile(sourcePath, kind)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	if info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("%s is not executable: %q", kind, sourcePath)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("%s must not be group or world writable: %q", kind, sourcePath)
	}
	if info.Size() < 1 || info.Size() > maxNativeExecutableBytes {
		return nil, fmt.Errorf("%s size %d is outside the allowed range", kind, info.Size())
	}
	var elfMagic [4]byte
	if _, err := io.ReadFull(source, elfMagic[:]); err != nil {
		return nil, fmt.Errorf("read %s header: %w", kind, err)
	}
	if elfMagic != [4]byte{0x7f, 'E', 'L', 'F'} {
		return nil, fmt.Errorf("%s must be an ELF executable", kind)
	}
	if err := validateStaticLinuxAMD64ELF(source, kind); err != nil {
		return nil, err
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind %s: %w", kind, err)
	}

	memfdFlags := unix.MFD_CLOEXEC | unix.MFD_ALLOW_SEALING | unix.MFD_EXEC
	memfd, err := unix.MemfdCreate(memfdName, memfdFlags)
	if err != nil {
		return nil, fmt.Errorf("create executable %s memfd with MFD_EXEC: %w", kind, err)
	}
	snapshot := os.NewFile(uintptr(memfd), memfdName)
	if snapshot == nil {
		unix.Close(memfd)
		return nil, fmt.Errorf("wrap %s memfd", kind)
	}
	keepSnapshot := false
	defer func() {
		if !keepSnapshot {
			snapshot.Close()
		}
	}()

	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(snapshot, h), source)
	if err != nil {
		return nil, fmt.Errorf("copy %s into sealed snapshot: %w", kind, err)
	}
	if written != info.Size() {
		return nil, fmt.Errorf("%s changed while snapshotting: read %d bytes, expected %d", kind, written, info.Size())
	}
	gotSHA256 := hex.EncodeToString(h.Sum(nil))
	if gotSHA256 != expectedSHA256 {
		return nil, fmt.Errorf("%s SHA-256 mismatch: want %s, got %s", kind, expectedSHA256, gotSHA256)
	}
	if err := unix.Fchmod(memfd, 0o500); err != nil {
		return nil, fmt.Errorf("make %s snapshot executable: %w", kind, err)
	}
	seals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_EXEC | unix.F_SEAL_SEAL
	if _, err := unix.FcntlInt(uintptr(memfd), unix.F_ADD_SEALS, seals); err != nil {
		return nil, fmt.Errorf("seal %s snapshot: %w", kind, err)
	}
	gotSeals, err := unix.FcntlInt(uintptr(memfd), unix.F_GET_SEALS, 0)
	if err != nil {
		return nil, fmt.Errorf("read %s snapshot seals: %w", kind, err)
	}
	if gotSeals&seals != seals {
		return nil, fmt.Errorf("%s snapshot seal mismatch: want %#x, got %#x", kind, seals, gotSeals)
	}

	keepSnapshot = true
	return snapshot, nil
}

func validateStaticLinuxAMD64ELF(source *os.File, kind string) error {
	parsed, err := elf.NewFile(source)
	if err != nil {
		return fmt.Errorf("parse %s ELF: %w", kind, err)
	}
	defer parsed.Close()
	if parsed.Class != elf.ELFCLASS64 || parsed.Machine != elf.EM_X86_64 {
		return fmt.Errorf(
			"%s must be a Linux x86-64 ELF executable, got class %s machine %s",
			kind,
			parsed.Class,
			parsed.Machine,
		)
	}
	if parsed.OSABI != elf.ELFOSABI_NONE && parsed.OSABI != elf.ELFOSABI_LINUX {
		return fmt.Errorf("%s must use the Linux ELF ABI, got %s", kind, parsed.OSABI)
	}
	if parsed.Type != elf.ET_EXEC && parsed.Type != elf.ET_DYN {
		return fmt.Errorf("%s must be an executable ELF, got type %s", kind, parsed.Type)
	}
	for _, program := range parsed.Progs {
		if program.Type == elf.PT_INTERP {
			return fmt.Errorf("%s must be statically linked: PT_INTERP is present", kind)
		}
	}
	imports, err := parsed.ImportedLibraries()
	if err != nil {
		return fmt.Errorf("inspect %s shared-library dependencies: %w", kind, err)
	}
	if len(imports) != 0 {
		return fmt.Errorf("%s must be statically linked: DT_NEEDED contains %v", kind, imports)
	}
	return nil
}
