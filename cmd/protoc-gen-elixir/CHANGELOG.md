# Changelog

## Unreleased

### Features

- Native Go reimplementation of the `protoc-gen-elixir` escript, targeting byte-for-byte compatible output against `elixir-protobuf/protobuf@a0ac409` (unreleased, pinned `mix.exs @version "0.17.0"`)
- Enums, messages (scalar fields, nested messages, oneofs, maps, proto3 `optional`), and services (`plugins=grpc`, including `.Stub` modules)
- proto2 extensions: both `extend` modules and `extensions [...]` ranges, with cross-file package aggregation
- `gen_descriptors`: generic Elixir-struct-literal pretty-printer for `google.protobuf.*` descriptor types
- `one_file_per_module`: one `.pb.ex` file per Elixir module instead of per `.proto` file
- `transform_module`: emits `def transform_module(), do: ...` on every generated message module
- `package_prefix`, `include_docs`, `gen_proto_source` parameter support
- `--elixir_opt` parameter parsing with strict-value validation and ldflags-based `--version` support

### Known limitations

- Custom proto2 extensions must be declared in `.proto` files passed to `protoc`; extensions defined only as compiled Elixir modules cannot be loaded (see [`README.md`](./README.md#extensions))
