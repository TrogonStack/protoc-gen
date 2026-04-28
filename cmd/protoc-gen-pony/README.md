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

v1 supports singular implicit-presence proto3 scalars (bool, int32/64,
uint32/64, sint32/64, fixed32/64, sfixed32/64, float, double, string,
bytes). Repeated fields, `optional` explicit presence, oneofs, maps,
embedded messages, and enums emit a `// TODO protoc-gen-pony` comment
until the corresponding codegen lands. Services (gRPC) are out of scope.

[buf]: https://buf.build
[runtime]: https://github.com/TrogonStack/protobuf-pony
