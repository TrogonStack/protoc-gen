# protoc-gen-pony

A Protobuf compiler plugin that generates Pony source code — `class val`
records plus sister `Codec` primitives that decode and encode against the
[`protobuf` Pony runtime library][runtime].

## Install

```bash
go install github.com/TrogonStack/protoc-gen/cmd/protoc-gen-pony@latest
```

## Usage

With `protoc`:

```bash
protoc --pony_out=gen path/to/file.proto
```

With [buf]:

```yaml
# buf.gen.yaml
version: v2
plugins:
  - local: protoc-gen-pony
    out: gen
```

## Output

For a `User` message in `acme/v1/user.proto`:

```protobuf
syntax = "proto3";
package acme.v1;

message User {
  int32 id = 1;
  string name = 2;
  bool active = 3;
}
```

The plugin writes `gen/acme/v1/user.pony` with a `class val User` record and a
`primitive UserCodec` exposing `decode(reader: WireReader ref): (User val |
WireError)` and `encode(writer: WireWriter ref, msg: User val)`.

## Runtime requirement

Generated code calls into the Pony `protobuf` package — `WireReader`,
`WireWriter`, `Tag`, `Scalar`, `WireType`, `WireError`. See [the runtime
sources][runtime].

## Coverage

Supported (no `TODO` comments emitted):

- All proto3 scalar types — bool, int32/64, uint32/64, sint32/64,
  fixed32/64, sfixed32/64, float, double, string, bytes
- Enums (primitives + type alias + `FromValue` dispatcher + `Raw` fallback)
- Singular and repeated embedded messages
- proto3 `optional` explicit presence (`(T | None)` type)
- Real `oneof` fields (wrapper class per member, union type alias)
- `map<K, V>` where V is a scalar, enum, or non-blocked message
- Cross-directory `use` directives (relative path, auto-deduped)
- Well-known types: Timestamp, Duration, Any, FieldMask, wrappers, Empty, etc.
  generate as regular proto3 messages with no special treatment

**Known limitations:**

- `google/protobuf/struct.proto`, `type.proto`, `api.proto`, `descriptor.proto`
  stay as `TODO` — circular or JSON-only types not representable in plain proto3
- JSON-specific WKT encoding (Timestamp as RFC 3339, etc.) is out of scope
- Services (gRPC stubs) are not generated

[buf]: https://buf.build
[runtime]: https://github.com/TrogonStack/protobuf-pony
