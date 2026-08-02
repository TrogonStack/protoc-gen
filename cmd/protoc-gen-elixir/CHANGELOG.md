# Changelog

## [0.1.1](https://github.com/TrogonStack/protoc-gen/compare/protoc-gen-elixir@v0.1.0...protoc-gen-elixir@v0.1.1) (2026-08-02)


### Bug Fixes

* **protoc-gen-elixir:** Emit one file per service under one_file_per_module ([#63](https://github.com/TrogonStack/protoc-gen/issues/63)) ([6efd221](https://github.com/TrogonStack/protoc-gen/commit/6efd221af6d35977e3161cdfc9600237c8b1f169))

## [0.1.0](https://github.com/TrogonStack/protoc-gen/compare/protoc-gen-elixir@v0.0.1...protoc-gen-elixir@v0.1.0) (2026-07-27)


### Features

* **protoc-gen-elixir:** Implement native Go code generation ([#58](https://github.com/TrogonStack/protoc-gen/issues/58)) ([dcb6e81](https://github.com/TrogonStack/protoc-gen/commit/dcb6e81b9dcff8c488570de333ed8b2680b44675))

## Changelog

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
