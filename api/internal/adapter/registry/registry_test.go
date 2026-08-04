package registry_test

import (
	"testing"

	"marrow/internal/adapter/registry"
)

func TestSourceAdapter_UnknownID_FailsLoud(t *testing.T) {
	_, err := registry.SourceAdapter("does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unregistered adapter id, got nil")
	}
}

func TestSourceAdapter_KnownID_Resolves(t *testing.T) {
	adp, err := registry.SourceAdapter("substack")
	if err != nil {
		t.Fatalf("expected substack adapter to resolve, got error: %v", err)
	}
	if adp.Id() != "substack" {
		t.Errorf("expected adapter id %q, got %q", "substack", adp.Id())
	}
}

// TestMediaResolver_RegisteredButWrongCapability_FailsLoud confirms that an
// adapter registered under an ID (substack) that doesn't implement
// MediaResolver produces a "does not implement" error, not a silent skip
// or a nil-interface panic.
func TestMediaResolver_RegisteredButWrongCapability_FailsLoud(t *testing.T) {
	_, err := registry.MediaResolver("substack")
	if err == nil {
		t.Fatal("expected an error for an adapter that doesn't implement MediaResolver, got nil")
	}
}

func TestMediaResolver_UnknownID_FailsLoud(t *testing.T) {
	_, err := registry.MediaResolver("does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unregistered adapter id, got nil")
	}
}
