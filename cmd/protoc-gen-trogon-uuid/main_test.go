package main

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGenerateFromDescriptorFixture(t *testing.T) {
	var descriptors descriptorpb.FileDescriptorSet
	require.NoError(t, proto.Unmarshal(testdataRecipes, &descriptors))
	for _, file := range descriptors.File {
		if file.GetName() != "example/catalog/v1/identity.proto" {
			continue
		}
		filteredEnums := file.EnumType[:0]
		for _, enum := range file.EnumType {
			if enum.GetName() == "MissingNamespaceVersion" {
				continue
			}
			filtered := enum.Value[:0]
			for _, value := range enum.Value {
				if strings.HasPrefix(value.GetName(), "BAD_") || value.GetName() == "EMPTY_NAMESPACE" {
					continue
				}
				filtered = append(filtered, value)
			}
			enum.Value = filtered
			filteredEnums = append(filteredEnums, enum)
		}
		file.EnumType = filteredEnums
	}

	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"example/catalog/v1/identity.proto"},
		ProtoFile:      descriptors.File,
	}
	response := generate(request)
	require.Empty(t, response.GetError())
	require.Len(t, response.GetFile(), 1)

	content := response.GetFile()[0].GetContent()
	require.Contains(t, content, "pub struct SkuIdentityVersionV1;")
	require.Contains(t, content, "pub struct SkuIdentityVersionV1Input<'a>")
	require.Contains(t, content, "trogon_uuid::uuid5_parts")
	require.Contains(t, content, "pub struct StaticTemplate;")
}

func TestGenerateReturnsDescriptorErrors(t *testing.T) {
	response := generate(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"missing.proto"},
	})
	require.NotEmpty(t, response.GetError())
}

//go:embed testdata/recipes.binpb
var testdataRecipes []byte
