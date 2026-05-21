package codeagent

import "testing"

func TestCatalogIncludesModelsForVerifiedProviders(t *testing.T) {
	t.Parallel()

	for provider, version := range VerifiedProviderVersions {
		entry, ok := Catalog(provider)
		if !ok {
			t.Fatalf("Catalog(%q) missing", provider)
		}
		if entry.Version != version {
			t.Fatalf("Catalog(%q).Version = %q, want %q", provider, entry.Version, version)
		}
		if entry.DefaultModel == "" {
			t.Fatalf("Catalog(%q).DefaultModel is empty", provider)
		}
		if len(entry.SupportedModels) == 0 {
			t.Fatalf("Catalog(%q).SupportedModels is empty", provider)
		}
		if !SupportsModel(provider, entry.DefaultModel) {
			t.Fatalf("Catalog(%q) default model %q is not supported", provider, entry.DefaultModel)
		}
	}
}

func TestSupportsModelRejectsUnknownModel(t *testing.T) {
	t.Parallel()

	if SupportsModel("codex", "not-a-tested-model") {
		t.Fatal("unexpected support for unknown model")
	}
}
