# protoc-gen-trogon

A protoc plugin for explicitly selected Trogon protobuf features and output
languages. The supported combination is currently `language=rust,features=uuid`.

## Usage

Install the executable:

```sh
go install github.com/TrogonStack/protoc-gen/cmd/protoc-gen-trogon@latest
```

Create the output directory, then select the language and generators:

```sh
mkdir -p gen
protoc --trogon_out=gen \
  --trogon_opt=language=rust,features=uuid \
  proto/identity.proto
```

With Buf:

```yaml
version: v2
plugins:
  - local: protoc-gen-trogon
    out: gen
    opt:
      - language=rust
      - features=uuid
```

The plugin uses `CodeGeneratorRequest` on stdin and `CodeGeneratorResponse` on
stdout. It does not infer a language or framework from other plugins.

## Options

- `language` is required. Only `rust` is supported.
- `features` is required. Only `uuid` is supported. Feature names use `+` as a
  list separator; duplicate or unavailable features are errors.
- `runtime_path` optionally changes the Rust runtime crate path, for example
  `runtime_path=::uuid_runtime`. It must be an absolute path of ordinary Rust
  identifiers; it does not choose a framework or output language.

Unknown options, missing values, duplicate selections and unsupported languages
or features produce an error response with no generated files. Generation
selection controls output, not the contents or size of the plugin executable.
There are no reduced build variants or dynamic plugins.

## Rust UUID recipes

The `uuid` feature reads the public `trogon.uuid.v1` enum options. For each
requested protobuf file with configured recipes, it writes a `.uuid.rs` file
beside the corresponding source-relative path. It inspects both top-level enums
and enums nested inside messages. Requested files without configured recipes
produce no output file. Imported files are available for option resolution;
only files requested by protoc produce output.

Each configured enum value emits a marker type using its underscore-separated
name converted to PascalCase. Generated marker and input names must be unique
within the output file. Invalid Rust names or collisions are generation errors.

A recipe containing `{field_name}` placeholders also emits a borrowed input
struct. Field names use ASCII letters, digits and underscores, cannot begin with
a digit, and cannot be `_`, `self`, `Self`, `super` or `crate`. Other Rust
keywords use raw identifiers, such as `r#type`. Repeated placeholders reuse the
same field and retain their order in the hash input. Literal Unicode and control
characters preserve their original UTF-8 bytes in the emitted Rust source.

Values without `(trogon.uuid.v1.enum_value)` are ignored, allowing unconfigured
sentinel values. A present option without `format` is an error. Empty templates
are valid constant recipes. Templates without placeholders emit a final `VALUE`
constant and accept `()` as input.

The value-level namespace overrides the enum default. A namespace must be supplied by the value or enum. A supplied namespace
without a selected UUID, DNS or URL form is an error. UUID, DNS and URL namespace forms
follow the published options. UUIDv5 first derives the version namespace using
the exact enum value name, then hashes the template's literal and input bytes in
order. Renaming a value or changing namespace/template bytes changes identity.
Input values are not normalized or escaped, so applications must validate values
where separators could make different logical inputs render the same name.

The generated code implements `trogon_uuid::UuidTemplate` and calls its
allocator-free runtime. It does not depend on a protobuf message framework,
standard library, allocator, random source or clock. Enable the runtime crate's
normal Rust dependency in the application that includes the generated files.

## Compatibility

Only Rust UUID recipes are implemented. Advertising proto3 optional support
allows recipe enums to coexist with optional message fields; the plugin does
not generate protobuf message implementations. Descriptor option wire shapes
are validated before code emission. Any failed file prevents all output for
that request.
