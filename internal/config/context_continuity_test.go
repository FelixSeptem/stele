package config

import "testing"

func TestPC4ContinuityConfigRejectsInvalid(t *testing.T) {
	for key, values := range map[string][]string{"TTL_SECONDS": {"0", "59", "604801"}, "MAX_RECEIPTS": {"0", "1000001"}, "MAX_RECEIPTS_PER_BINDING": {"0", "4097"}, "CLEANUP_BATCH": {"0", "5001"}} {
		for _, v := range values {
			t.Run(key+v, func(t *testing.T) {
				t.Setenv("STELE_PROVIDER_CONTEXT_CONTINUITY_"+key, v)
				if _, err := loadProviderConfig(); err == nil {
					t.Fatal("invalid continuity config accepted")
				}
			})
		}
	}
	t.Run("inconsistent_capacity", func(t *testing.T) {
		t.Setenv("STELE_PROVIDER_CONTEXT_CONTINUITY_MAX_RECEIPTS", "1")
		if _, err := loadProviderConfig(); err == nil {
			t.Fatal("per binding exceeds global")
		}
	})
}
