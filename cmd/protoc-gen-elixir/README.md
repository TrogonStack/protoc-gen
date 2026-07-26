# protoc-gen-elixir

A native Go reimplementation of the `protoc-gen-elixir` escript published by
[`elixir-protobuf/protobuf`](https://github.com/elixir-protobuf/protobuf),
targeting byte-for-byte compatible output — so Elixir projects can swap their
codegen toolchain (escript → single static binary) without changing a single
generated file.

**Status: feature-complete.** Enums, messages (including nested messages,
oneofs, and maps), services, extensions, `gen_descriptors`,
`one_file_per_module`, and `transform_module` are all implemented and
verified byte-for-byte against the reference escript's golden fixtures.

## Install

```bash
go install github.com/TrogonStack/protoc-gen/cmd/protoc-gen-elixir@latest
```

## Usage

With `protoc`:

```bash
protoc --elixir_out=lib path/to/file.proto
protoc --elixir_out=plugins=grpc:lib path/to/file.proto
```

With [buf]:

```yaml
# buf.gen.yaml
version: v2
plugins:
  - local: protoc-gen-elixir
    out: lib
```

## Parameters

Passed as a comma-separated `key=value` list via `--elixir_opt=...` (or
`opt:` entries under `buf.gen.yaml`). Unknown keys are silently ignored,
matching the reference escript.

| Flag                  | Value                | Behavior                                                                                              |
|-----------------------|-----------------------|--------------------------------------------------------------------------------------------------------|
| `plugins`             | e.g. `grpc`           | Sub-plugins joined by `+`. Only `grpc` is meaningful today; enables service module emission.           |
| `gen_descriptors`     | `"true"`              | Emits a `descriptor()` function on every message/enum/service module.                                  |
| `package_prefix`      | non-empty string      | Prepended to the proto package when computing Elixir module names.                                     |
| `transform_module`    | string                | Emits `def transform_module(), do: ...` on every generated message module.                             |
| `one_file_per_module` | `"true"`              | Emits one `.pb.ex` file per Elixir module instead of per `.proto` file.                                 |
| `include_docs`        | `"true"`              | Emits `@moduledoc` from proto source comments.                                                         |
| `gen_proto_source`    | `"true"`              | Embeds the originating `.proto` file path on message and service modules.                              |

`gen_descriptors`, `one_file_per_module`, `include_docs`, and
`gen_proto_source` only accept the literal value `"true"` — any other value
is a `CodeGeneratorResponse` error. `package_prefix` rejects an empty value.

## Extensions

If you use proto2 extensions, pass the extension-defining `.proto` files to
`protoc` (via `-I` include paths) the same way you do with any other
dependency. We cannot load custom extensions from compiled Elixir modules —
that is a fundamental Elixir runtime feature, not a protoc-plugin feature.

[buf]: https://buf.build
