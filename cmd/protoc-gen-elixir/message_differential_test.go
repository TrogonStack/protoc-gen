package main

import "testing"

// TestMessageDifferential_Extension covers extension.proto end-to-end: its
// only message (Foo) is a flat, single-scalar-field, top-level message, so
// the whole file is within Phase 1+2's combined scope (unlike test.proto,
// which has nested/oneof/map/extension constructs still out of scope).
func TestMessageDifferential_Extension(t *testing.T) {
	t.Parallel()

	assertMatchesReference(t, "include_docs=true", "extension.proto")
}

// TestMessageDifferential_NoPackage covers no_package.proto end-to-end: its
// only message (NoPackageMessage) has a single map field and no extensions,
// so - unlike test.proto, which still has Phase 5 extension constructs out
// of scope - the whole file is within Phase 1+3's combined scope. This is
// the "true whole-file differential test" fixture Phase 3 needed: it
// exercises the synthesized map-entry nested message (NumberMappingEntry),
// its post-order placement before its parent's body, and the map field's
// trailing "map: true" option, all in one file with no Phase 5 gate needed.
func TestMessageDifferential_NoPackage(t *testing.T) {
	t.Parallel()

	assertMatchesReference(t, "include_docs=true", "no_package.proto")
}

// TestServiceDifferential_GRPC covers service.proto + test.proto end-to-end
// under plugins=grpc,include_docs=true, mirroring testdata/gen_goldens.sh's
// own `gen grpc` invocation. service.proto imports test.proto and
// cross-references its Request/Reply messages, so this exercises
// TypeRegistry's cross-file resolution as well as the .Service/.Stub module
// pair end-to-end against the real escript, not just the pinned golden
// fixture snapshot.
func TestServiceDifferential_GRPC(t *testing.T) {
	t.Parallel()

	assertMatchesReference(t, "plugins=grpc,include_docs=true", "test.proto", "service.proto")
}
