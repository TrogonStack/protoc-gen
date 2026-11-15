# protoc-gen-trogon-uuid

Generates Rust `trogon_uuid::UuidTemplate` implementations from the public
`trogon.uuid.v1` protobuf options.

## Usage

Build the plugin and make it available on `PATH` as `protoc-gen-trogon-uuid`:

```sh
protoc --trogon-uuid_out=gen proto/identity.proto
```

The plugin emits one `.uuid.rs` file per generated protobuf file. Values without
`(trogon.uuid.v1.enum_value).format` are ignored. Configured values resolve their
namespace from the value-level option first, then the enum-level option. The
version namespace is `UUIDv5(namespace, enum_value_name)`.

Templates with `{field_name}` placeholders emit a marker, a borrowed input
struct, and a streaming `uuid5` implementation. Templates without placeholders
emit a `VALUE` constant. The generated code depends only on the core
`trogon_uuid` runtime.

The plugin receives `CodeGeneratorRequest` on stdin and writes
`CodeGeneratorResponse` on stdout, so it works with both `protoc` and Buf.

## Runtime path

Use `--trogon-uuid_opt=runtime_path=::my_uuid_runtime` when the runtime crate has
a renamed dependency.

## Limitations

The plugin currently emits Rust recipes only. It validates the UUID namespace
option shape, templates and generated identifiers, and reports malformed
configured options through the protoc response.
