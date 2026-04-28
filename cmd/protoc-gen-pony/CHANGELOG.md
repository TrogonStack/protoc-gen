# Changelog

## Unreleased

### Features

* Initial release. Generates Pony `class val` records + sister `Codec`
  primitives for proto3 messages with singular implicit-presence scalar
  fields. Repeated, `optional`, oneof, map, embedded-message, and enum
  fields surface as `// TODO protoc-gen-pony` placeholders.
