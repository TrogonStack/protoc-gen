# Changelog

## Unreleased

### Features

- All proto3 scalar types (bool, int32/64, uint32/64, sint32/64, fixed32/64, sfixed32/64, float, double, string, bytes)
- Enums: primitive per value, type alias union, `FromValue` dispatcher, `Raw` class for unknown values
- Singular and repeated embedded messages; sub-codec decode/encode
- Repeated scalar and enum fields (packed wire format)
- proto3 `optional` explicit presence (`(T | None)` type, match-on-None encode)
- Real `oneof` fields: wrapper class per member, union type alias, full decode/encode
- `map<K, V>` fields: scalar, enum, and message values; `use "collections"` auto-emitted
- Cross-directory `use` directives (relative paths, deduplicated per directory)
- Well-known types (`google/protobuf/timestamp.proto`, `duration.proto`, `any.proto`, `wrappers.proto`, `field_mask.proto`, `empty.proto`, etc.) generate as regular proto3 messages
- Generated file header includes minimum required `protobuf-pony` runtime version

### Known limitations

- `google/protobuf/struct.proto`, `type.proto`, `api.proto`, `descriptor.proto` emit `TODO` comments — circular or JSON-only semantics
- JSON-specific WKT encoding (Timestamp as RFC 3339, etc.) is out of scope
- Services (gRPC stubs) are not generated
