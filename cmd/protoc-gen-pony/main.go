// protoc-gen-pony is a plugin for the Protobuf compiler that generates
// Pony code (class val records + sister Codec primitives that decode/encode
// against the Pony `protobuf` runtime library). To use it, build this
// program and make it available on your PATH as protoc-gen-pony.
//
// With protoc:
//
//	protoc --pony_out=gen path/to/file.proto
//
// With [buf], your buf.gen.yaml will look like this:
//
//	version: v2
//	plugins:
//	  - local: protoc-gen-pony
//	    out: gen
//
// Generated files import the `protobuf` runtime library — see
// https://github.com/TrogonStack/protobuf-pony for the Pony source. The
// runtime exposes WireReader/WireWriter, Tag, Scalar, the WireType union,
// and the WireError typed-error union.
//
// [buf]: https://buf.build
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	// Set by ldflags during build time.
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const usage = "\n\nFlags:\n  -h, --help\tPrint this help and exit.\n      --version\tPrint the version and exit."

// goImportStub keeps protogen happy on non-Go targets. protogen.Options.New
// errors out if any input file lacks a Go import path; users targeting Pony
// shouldn't have to set go_package or M-mappings just for that. We prepend
// a stub M-entry for every file before calling protogen — user-provided
// params still win because they appear later in the Parameter string.
const goImportStub = "protoc-gen-pony/stub"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("protoc-gen-pony %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		if _, err := fmt.Fprintln(os.Stdout, usage); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if len(os.Args) != 1 {
		if _, err := fmt.Fprintln(os.Stderr, usage); err != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}

	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-pony: read stdin: %v\n", err)
		os.Exit(1)
	}
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(in, &req); err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-pony: unmarshal request: %v\n", err)
		os.Exit(1)
	}
	injectGoImportStubs(&req)

	var flagSet flag.FlagSet
	plugin, err := protogen.Options{ParamFunc: flagSet.Set}.New(&req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-pony: %v\n", err)
		os.Exit(1)
	}
	plugin.SupportedFeatures =
		uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL) |
			uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
	plugin.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_PROTO2
	plugin.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2024

	for _, file := range plugin.Files {
		if file.Generate {
			generateFile(plugin, file)
		}
	}

	resp := plugin.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-pony: marshal response: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-pony: write stdout: %v\n", err)
		os.Exit(1)
	}
}

func injectGoImportStubs(req *pluginpb.CodeGeneratorRequest) {
	parts := make([]string, 0, len(req.GetProtoFile())+1)
	for _, file := range req.GetProtoFile() {
		parts = append(parts, "M"+file.GetName()+"="+goImportStub)
	}
	if existing := req.GetParameter(); existing != "" {
		parts = append(parts, existing)
	}
	combined := strings.Join(parts, ",")
	req.Parameter = &combined
}
