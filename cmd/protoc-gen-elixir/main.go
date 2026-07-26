// protoc-gen-elixir is a plugin for the Protobuf compiler that generates
// Elixir modules from .proto message, enum, and service definitions. It is a
// native Go reimplementation of the protoc-gen-elixir escript published by
// [elixir-protobuf/protobuf], byte-compatible with its current HEAD (not yet
// tagged; mix.exs @version "0.17.0").
//
// To use it, build this program and make it available on your PATH as
// protoc-gen-elixir:
//
//	protoc --elixir_out=lib path/to/file.proto
//	protoc --elixir_out=plugins=grpc:lib path/to/file.proto
//
// [elixir-protobuf/protobuf]: https://github.com/elixir-protobuf/protobuf
package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	// These variables are set by ldflags during build time.
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const usage = "\n\nFlags:\n" +
	"  -h, --help\tPrint this help and exit.\n" +
	"      --version\tPrint the version and exit.\n\n" +
	"Plugin parameters (--elixir_opt=key=value,...):\n" +
	"      plugins\tSub-plugins joined by '+'. Only \"grpc\" is meaningful; enables service module emission.\n" +
	"      gen_descriptors\tWhen \"true\", emit a descriptor() function on every message/enum/service module.\n" +
	"      package_prefix\tPrepended to the proto package when computing Elixir module names. Cannot be empty.\n" +
	"      transform_module\tModule name emitted via transform_module() on every generated message module.\n" +
	"      one_file_per_module\tWhen \"true\", emit one .pb.ex file per Elixir module instead of per .proto file.\n" +
	"      include_docs\tWhen \"true\", emit @moduledoc from proto source comments.\n" +
	"      gen_proto_source\tWhen \"true\", embed the originating .proto file path in every module."

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("protoc-gen-elixir %s (commit: %s, built: %s)\n", version, commit, date)
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

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %v\n", err)
		os.Exit(1)
	}

	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(input, &req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %v\n", err)
		os.Exit(1)
	}

	if err := writeResponse(os.Stdout, &req); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func writeResponse(w io.Writer, req *pluginpb.CodeGeneratorRequest) error {
	output, err := proto.Marshal(generate(req))
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if _, err := w.Write(output); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}
