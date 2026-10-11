package config

import "testing"

func TestProviderContextBudgetConfiguration(t *testing.T) {
	t.Setenv("STELE_PROVIDER_MAX_CONTEXT_ITEMS", "100")
	t.Setenv("STELE_PROVIDER_MAX_CONTEXT_BYTES", "8192")
	t.Setenv("STELE_PROVIDER_MAX_CONTEXT_CONTENT_BYTES", "")
	c, err := loadProviderConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Limits.MaxContextContentBytes != 8192 || c.Limits.MaxContextItems != 100 {
		t.Fatal("independent inherited defaults")
	}
	t.Setenv("STELE_PROVIDER_MAX_CONTEXT_CONTENT_BYTES", "0")
	c, err = loadProviderConfig()
	if err != nil || c.Limits.MaxContextContentBytes != 0 {
		t.Fatal("zero content config")
	}
	for _, x := range []struct{ key, value string }{{"STELE_PROVIDER_MAX_CONTEXT_ITEMS", "0"}, {"STELE_PROVIDER_MAX_CONTEXT_ITEMS", "1001"}, {"STELE_PROVIDER_MAX_CONTEXT_CONTENT_BYTES", "8193"}, {"STELE_PROVIDER_MAX_CONTEXT_CONTENT_BYTES", "-1"}, {"STELE_PROVIDER_MAX_CONTEXT_BYTES", "0"}, {"STELE_PROVIDER_MAX_CITATIONS", "0"}} {
		t.Run(x.key+x.value, func(t *testing.T) {
			t.Setenv(x.key, x.value)
			if _, err := loadProviderConfig(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
