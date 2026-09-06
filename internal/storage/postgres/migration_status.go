package postgres

type MigrationState string

const (
	MigrationStateCurrent      MigrationState = "current"
	MigrationStatePending      MigrationState = "pending"
	MigrationStateDirty        MigrationState = "dirty"
	MigrationStateDivergent    MigrationState = "divergent"
	MigrationStateIncompatible MigrationState = "incompatible"
)

func (s MigrationState) Valid() bool {
	switch s {
	case MigrationStateCurrent, MigrationStatePending, MigrationStateDirty, MigrationStateDivergent, MigrationStateIncompatible:
		return true
	default:
		return false
	}
}

type MigrationErrorCategory string

const (
	MigrationErrorCategoryPending      MigrationErrorCategory = "pending"
	MigrationErrorCategoryDirty        MigrationErrorCategory = "dirty"
	MigrationErrorCategoryDivergent    MigrationErrorCategory = "divergent"
	MigrationErrorCategoryIncompatible MigrationErrorCategory = "incompatible"
	MigrationErrorCategoryLock         MigrationErrorCategory = "lock"
	MigrationErrorCategoryApply        MigrationErrorCategory = "apply"
)

func (c MigrationErrorCategory) Valid() bool {
	switch c {
	case MigrationErrorCategoryPending, MigrationErrorCategoryDirty, MigrationErrorCategoryDivergent, MigrationErrorCategoryIncompatible, MigrationErrorCategoryLock, MigrationErrorCategoryApply:
		return true
	default:
		return false
	}
}

type MigrationStatus struct {
	State           MigrationState `json:"state"`
	CurrentVersion  int64          `json:"current_version"`
	RequiredVersion int64          `json:"required_version"`
	Dirty           bool           `json:"dirty"`
	PendingVersions []int64        `json:"pending_versions,omitempty"`
}
