#!/usr/bin/env bash
set -euo pipefail

export LC_ALL=C
export TZ=UTC
umask 077

readonly PINNED_NSJAIL_COMMIT="f78475530b46d0186111a9096b30725f816b55fe"
readonly PINNED_KAFEL_COMMIT="76d0f41bf3eb5c4008713d64b9767b461a9129a3"

usage() {
  echo "usage: $0 /absolute/path/to/nsjail-checkout /absolute/path/to/nsjail-static" >&2
}

if [[ $# -ne 2 ]]; then
  usage
  exit 64
fi

source_root="$(realpath "$1")"
output_path="$2"
if [[ "$output_path" != /* ]]; then
  echo "output path must be absolute: $output_path" >&2
  exit 64
fi
if [[ -e "$output_path" ]]; then
  echo "refusing to replace existing output: $output_path" >&2
  exit 73
fi
output_dir="$(dirname "$output_path")"
if [[ ! -d "$output_dir" ]]; then
  echo "output directory does not exist: $output_dir" >&2
  exit 72
fi
if [[ "$(git -C "$source_root" rev-parse --show-toplevel)" != "$source_root" ]]; then
  echo "source path must be the NsJail repository root" >&2
  exit 65
fi
if [[ "$(git -C "$source_root" rev-parse HEAD)" != "$PINNED_NSJAIL_COMMIT" ]]; then
  echo "NsJail checkout is not at pinned commit $PINNED_NSJAIL_COMMIT" >&2
  exit 65
fi
if [[ ! -d "$source_root/kafel/.git" && ! -f "$source_root/kafel/.git" ]]; then
  echo "the pinned Kafel submodule must already be initialized" >&2
  exit 65
fi
if [[ "$(git -C "$source_root/kafel" rev-parse HEAD)" != "$PINNED_KAFEL_COMMIT" ]]; then
  echo "Kafel checkout is not at pinned commit $PINNED_KAFEL_COMMIT" >&2
  exit 65
fi
if ! git -C "$source_root" diff --quiet HEAD -- ||
  ! git -C "$source_root" diff --cached --quiet -- ||
  ! git -C "$source_root/kafel" diff --quiet HEAD -- ||
  ! git -C "$source_root/kafel" diff --cached --quiet --; then
  echo "NsJail and Kafel tracked sources must be clean" >&2
  exit 65
fi

for tool in make g++ gcc git tar pkg-config protoc flex bison ld ar objdump objcopy \
  readelf file sha256sum awk grep sed realpath mktemp; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required build tool is unavailable: $tool" >&2
    exit 69
  fi
done
if ! pkg-config --exists protobuf libnl-route-3.0; then
  echo "static protobuf and libnl-route development dependencies are required" >&2
  exit 69
fi

build_jobs="${NSJAIL_BUILD_JOBS:-2}"
if [[ ! "$build_jobs" =~ ^[1-9][0-9]*$ ]]; then
  echo "NSJAIL_BUILD_JOBS must be a positive integer" >&2
  exit 64
fi

build_parent="$(mktemp -d "$output_dir/.nsjail-build.XXXXXX")"
build_root="$build_parent/source"
temporary_output="$build_parent/nsjail-static"
cleanup() {
  rm -rf -- "$build_parent"
}
trap cleanup EXIT

mkdir -p "$build_root/kafel"
git -C "$source_root" archive --format=tar HEAD | tar -xf - -C "$build_root"
git -C "$source_root/kafel" archive --format=tar HEAD | tar -xf - -C "$build_root/kafel"
export SOURCE_DATE_EPOCH="$(git -C "$source_root" show -s --format=%ct HEAD)"
build_cxx="${CXX:-g++}"
build_cc="${CC:-gcc}"
make -C "$build_root" -j "$build_jobs" CXX="$build_cxx" CC="$build_cc"

objects=(
  caps.o cgroup.o cgroup2.o cmdline.o config.o contain.o cpu.o logs.o
  mnt.o mnt_legacy.o mnt_newapi.o net.o nsjail.o pid.o sandbox.o subproc.o
  uts.o user.o util.o config.pb.o
)
static_libraries="$(pkg-config --libs --static protobuf libnl-route-3.0)"
read -r -a static_library_args <<<"$static_libraries"
(
  cd "$build_root"
  "$build_cxx" -static -o "$temporary_output" "${objects[@]}" kafel/libkafel.a \
    -pie -Wl,-z,noexecstack "${static_library_args[@]}"
)
chmod 0555 "$temporary_output"

program_headers="$(readelf -l "$temporary_output")"
dynamic_section="$(readelf -d "$temporary_output")"
file_identity="$(file "$temporary_output")"
if [[ "$program_headers" == *INTERP* ]]; then
  echo "static NsJail artifact unexpectedly contains PT_INTERP" >&2
  exit 70
fi
if [[ "$dynamic_section" == *NEEDED* ]]; then
  echo "static NsJail artifact unexpectedly contains DT_NEEDED" >&2
  exit 70
fi
if [[ ! "$file_identity" =~ ELF\ 64-bit.*x86-64.*statically\ linked ]]; then
  echo "output is not a static Linux x86-64 ELF" >&2
  exit 70
fi
if ! "$temporary_output" --help >/dev/null 2>&1; then
  echo "static NsJail artifact failed its local help probe" >&2
  exit 70
fi

actual_sha256="$(sha256sum "$temporary_output" | awk '{print $1}')"
if [[ -n "${NSJAIL_EXPECTED_SHA256:-}" && "$actual_sha256" != "$NSJAIL_EXPECTED_SHA256" ]]; then
  echo "NsJail SHA-256 mismatch: want $NSJAIL_EXPECTED_SHA256, got $actual_sha256" >&2
  exit 70
fi

mv -- "$temporary_output" "$output_path"
rm -rf -- "$build_parent"
trap - EXIT
echo "NsJail commit: $PINNED_NSJAIL_COMMIT"
echo "Kafel commit: $PINNED_KAFEL_COMMIT"
echo "SHA-256: $actual_sha256"
echo "Artifact: $output_path"
