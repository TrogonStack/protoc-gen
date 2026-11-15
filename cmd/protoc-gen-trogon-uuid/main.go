// protoc-gen-trogon-uuid generates Rust UUIDv5 recipes from trogon.uuid.v1
// enum options. Build this program as protoc-gen-trogon-uuid and invoke it
// with protoc --trogon-uuid_out=... input.proto.
package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("protoc-gen-trogon-uuid %s (commit: %s, built: %s)\n", version, commit, date)
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		_, _ = fmt.Fprintln(os.Stdout, "protoc-gen-trogon-uuid generates Rust UUIDv5 recipes.")
		_, _ = fmt.Fprintln(os.Stdout, "Usage: protoc --trogon-uuid_out=OUT input.proto")
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "protoc-gen-trogon-uuid: unexpected arguments")
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError("read CodeGeneratorRequest: %v", err)
		return
	}
	var request pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(input, &request); err != nil {
		writeError("decode CodeGeneratorRequest: %v", err)
		return
	}

	response := generate(&request)
	output, err := proto.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-trogon-uuid: encode CodeGeneratorResponse: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(output); err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-trogon-uuid: write response: %v\n", err)
		os.Exit(1)
	}
}

func generate(request *pluginpb.CodeGeneratorRequest) *pluginpb.CodeGeneratorResponse {
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{File: request.ProtoFile})
	if err != nil {
		return &pluginpb.CodeGeneratorResponse{Error: proto.String(fmt.Sprintf("load descriptor set: %v", err))}
	}
	response := &pluginpb.CodeGeneratorResponse{}
	for _, path := range request.FileToGenerate {
		file, err := files.FindFileByPath(path)
		if err != nil {
			response.Error = proto.String(fmt.Sprintf("find %s: %v", path, err))
			break
		}
		content, err := generateFile(file, files, request.GetParameter())
		if err != nil {
			response.Error = proto.String(fmt.Sprintf("%s: %v", path, err))
			break
		}
		if content != "" {
			response.File = append(response.File, &pluginpb.CodeGeneratorResponse_File{
				Name:    proto.String(outputName(path)),
				Content: proto.String(content),
			})
		}
	}
	return response
}

func writeError(format string, args ...any) {
	response := &pluginpb.CodeGeneratorResponse{Error: proto.String(fmt.Sprintf(format, args...))}
	output, err := proto.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-trogon-uuid: %v\n", err)
		return
	}
	_, _ = os.Stdout.Write(output)
}

func outputName(path string) string {
	if len(path) >= len(".proto") && path[len(path)-len(".proto"):] == ".proto" {
		return path[:len(path)-len(".proto")] + ".uuid.rs"
	}
	return path + ".uuid.rs"
}
