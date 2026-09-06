package postgres

import (
	"encoding/json"
	"testing"
)

func TestMigrationStatusAndErrorCategoriesAreStableAndSerializable(t *testing.T) {
	status := MigrationStatus{
		State:           MigrationStatePending,
		CurrentVersion:  1,
		RequiredVersion: 2,
		Dirty:           false,
		PendingVersions: []int64{2},
	}
	for _, state := range []MigrationState{
		MigrationStateCurrent,
		MigrationStatePending,
		MigrationStateDirty,
		MigrationStateDivergent,
		MigrationStateIncompatible,
	} {
		if !state.Valid() {
			t.Fatalf("migration state %q is not valid", state)
		}
	}
	for _, category := range []MigrationErrorCategory{
		MigrationErrorCategoryPending,
		MigrationErrorCategoryDirty,
		MigrationErrorCategoryDivergent,
		MigrationErrorCategoryIncompatible,
		MigrationErrorCategoryLock,
		MigrationErrorCategoryApply,
	} {
		if !category.Valid() {
			t.Fatalf("migration error category %q is not valid", category)
		}
	}
	if _, err := json.Marshal(status); err != nil {
		t.Fatalf("json.Marshal(MigrationStatus) error = %v", err)
	}
}
