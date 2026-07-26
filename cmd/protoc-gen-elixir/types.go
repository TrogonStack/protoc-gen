package main

import (
	"google.golang.org/protobuf/types/descriptorpb"
)

// TypeRegistry resolves a fully-qualified proto type name (as found in
// FieldDescriptorProto.GetTypeName(), e.g. ".test.Request.Color") to the
// Elixir module name that generating that type's own file would produce.
//
// It is built once per plugin invocation by walking every file in the
// CodeGeneratorRequest (req.GetProtoFile(), not just file_to_generate - a
// referenced type may live in an imported file that isn't itself being
// generated), recursively indexing every message/enum at every nesting
// depth by its fully-qualified proto name.
//
// package_prefix only applies to files actually being generated this run
// (file_to_generate), not to transitive imports. So a type defined in a file
// that is NOT in file_to_generate must resolve to a module name computed
// WITHOUT package_prefix, even though the same package_prefix value is in
// effect for the overall plugin invocation.
//
// Transitive `import public` resolution is not implemented here: no fixture
// in this phase's corpus exercises it, and
// Phase 3's own test corpus resolves entirely within a single file. This is
// a known gap, left for whenever a fixture actually needs it.
type TypeRegistry struct {
	modNames   map[string]string
	mapEntries map[string]bool
}

// NewTypeRegistry builds a TypeRegistry from every file in protoFiles.
// fileToGenerate is the set of proto file names (FileDescriptorProto.Name)
// that are being generated this run; only those files have packagePrefix
// applied when computing their types' module names.
func NewTypeRegistry(protoFiles []*descriptorpb.FileDescriptorProto, fileToGenerate map[string]bool, packagePrefix *string) *TypeRegistry {
	reg := &TypeRegistry{
		modNames:   make(map[string]string),
		mapEntries: make(map[string]bool),
	}

	for _, file := range protoFiles {
		effectivePrefix := packagePrefix
		if !fileToGenerate[file.GetName()] {
			effectivePrefix = nil
		}

		baseModName := ModName(file, effectivePrefix, nil)
		pkg := file.GetPackage()

		for _, enum := range file.GetEnumType() {
			reg.indexEnum(enum, baseModName, pkg)
		}
		for _, msg := range file.GetMessageType() {
			reg.indexMessage(msg, baseModName, pkg)
		}
	}

	return reg
}

func (r *TypeRegistry) indexEnum(enum *descriptorpb.EnumDescriptorProto, parentModName, parentFullName string) {
	modName := qualifyModName(parentModName, CamelizeEach(enum.GetName()))
	fullName := qualifyFullName(parentFullName, enum.GetName())
	r.modNames["."+fullName] = modName
}

func (r *TypeRegistry) indexMessage(msg *descriptorpb.DescriptorProto, parentModName, parentFullName string) {
	modName := qualifyModName(parentModName, CamelizeEach(msg.GetName()))
	fullName := qualifyFullName(parentFullName, msg.GetName())
	r.modNames["."+fullName] = modName
	if msg.GetOptions().GetMapEntry() {
		r.mapEntries["."+fullName] = true
	}

	for _, enum := range msg.GetEnumType() {
		r.indexEnum(enum, modName, fullName)
	}
	for _, nested := range msg.GetNestedType() {
		r.indexMessage(nested, modName, fullName)
	}
}

// Resolve looks up the Elixir module name for a fully-qualified proto type
// name as it appears in FieldDescriptorProto.GetTypeName() (leading-dot
// form, e.g. ".test.Request.Color"). Returns ("", false) if the type isn't
// found in the registry.
func (r *TypeRegistry) Resolve(typeName string) (string, bool) {
	modName, ok := r.modNames[typeName]
	return modName, ok
}

// IsMapEntry reports whether typeName (leading-dot form, as it appears in
// FieldDescriptorProto.GetTypeName()) refers to a synthesized map-entry
// message (options.map_entry == true).
func (r *TypeRegistry) IsMapEntry(typeName string) bool {
	return r.mapEntries[typeName]
}

// IsMapField reports whether field is a map field per the map fields rule:
// LABEL_REPEATED, TYPE_MESSAGE, and the referenced message type has
// options.map_entry == true.
func (r *TypeRegistry) IsMapField(field *descriptorpb.FieldDescriptorProto) bool {
	if field.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return false
	}
	if field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return false
	}
	return r.IsMapEntry(field.GetTypeName())
}

// buildFileToGenerateSet converts a file_to_generate name list into a set
// for O(1) membership checks in NewTypeRegistry.
func buildFileToGenerateSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}
