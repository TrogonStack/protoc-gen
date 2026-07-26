#!/usr/bin/env bash
# Regenerates cmd/protoc-gen-elixir/testdata/golden/ by running the real
# elixir-protobuf/protobuf escript (built from the pinned HEAD commit
# documented in ../TODO.md) against ../testdata/proto/.
#
# Requires:
#   - protoc on PATH.
#   - PROTOC_GEN_ELIXIR_ESCRIPT: path to a built `protoc-gen-elixir` escript
#     from an elixir-protobuf/protobuf checkout at the pinned commit
#     (`cd <checkout> && MIX_ENV=prod mix escript.build`).
#   - GOOGLE_PROTOBUF_SRC: path to a protobuf source checkout's `src/`
#     directory (contains `google/protobuf/descriptor.proto`), needed for
#     fixtures that import it. If unset, defaults to
#     <checkout>/deps/google_protobuf/src relative to the escript's dir.
set -euo pipefail

: "${PROTOC_GEN_ELIXIR_ESCRIPT:?set PROTOC_GEN_ELIXIR_ESCRIPT to a built protoc-gen-elixir escript}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="$SCRIPT_DIR/proto"
GOLDEN_DIR="$SCRIPT_DIR/golden"

GOOGLE_PROTOBUF_SRC="${GOOGLE_PROTOBUF_SRC:-$(dirname "$PROTOC_GEN_ELIXIR_ESCRIPT")/deps/google_protobuf/src}"

rm -rf "$GOLDEN_DIR"
mkdir -p "$GOLDEN_DIR"

gen() {
  local name="$1"
  local opt="$2"
  shift 2
  local out="$GOLDEN_DIR/$name"
  mkdir -p "$out"
  protoc \
    -I "$PROTO_DIR" \
    -I "$GOOGLE_PROTOBUF_SRC" \
    --plugin="protoc-gen-elixir=$PROTOC_GEN_ELIXIR_ESCRIPT" \
    --elixir_out="$([ -n "$opt" ] && echo "$opt:")$out" \
    "$@"
}

# Mirrors elixir-protobuf/protobuf's own mix.exs `gen_test_protos` alias.
gen extension "include_docs=true" "$PROTO_DIR/extension.proto"
gen package_prefix "package_prefix=my,include_docs=true" "$PROTO_DIR/test.proto" "$PROTO_DIR/service.proto"
gen gen_descriptors "gen_descriptors=true,include_docs=true" "$PROTO_DIR/custom_options.proto"
gen no_package "include_docs=true" "$PROTO_DIR/no_package.proto"
gen full_name "gen_proto_source=true,include_docs=true" "$PROTO_DIR/full_name.proto"

# Additions beyond upstream's own fixture generation (see TODO.md's "Reference
# Test-Suite Coverage Map" and "Test / Validation Plan"):
gen grpc "plugins=grpc,include_docs=true" "$PROTO_DIR/test.proto" "$PROTO_DIR/service.proto"
gen grpc_proto_source "plugins=grpc,gen_proto_source=true" "$PROTO_DIR/test.proto" "$PROTO_DIR/service.proto"
gen transform_module "transform_module=My.App.Transform" "$PROTO_DIR/test.proto"
gen one_file_per_module "one_file_per_module=true,include_docs=true" "$PROTO_DIR/test.proto" "$PROTO_DIR/service.proto"
gen path_doubling "" "$PROTO_DIR/foo/bar/mirror.proto"

echo "Goldens regenerated under $GOLDEN_DIR"
