# Test corpus provenance

These `.proto` files are lifted verbatim from
[`elixir-protobuf/protobuf`](https://github.com/elixir-protobuf/protobuf)
(MIT licensed) at the HEAD commit pinned in `../TODO.md`:

- `custom_options.proto`, `extension.proto`, `full_name.proto`,
  `no_package.proto`, `service.proto`, `test.proto` — from
  `test/protobuf/protoc/proto/`.
- `elixirpb.proto` — from `src/elixirpb.proto` (the plugin's own
  `module_prefix` file-option extension). Also published on the Buf
  Schema Registry at `buf.build/elixir-protobuf/protobuf` (package
  `elixirpb`) — we keep our own local copy here for tests rather than
  taking on a BSR dependency.

Kept unmodified so golden output can be diffed directly against the
reference escript's generation of the same inputs. If upstream changes
these fixtures, re-lift rather than hand-edit.
