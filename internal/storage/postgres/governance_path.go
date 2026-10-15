package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

// GetLatestCanonicalByScopePathAndClass keeps consolidation within the exact
// path asserted by its candidate, including when a sibling path is newer.
func (r *Repository) GetLatestCanonicalByScopePathAndClass(ctx context.Context, scope memory.Scope, path string, class memory.MemoryClass) (memory.CanonicalMemory, bool, error) {
	if err := scope.Validate(); err != nil {
		return memory.CanonicalMemory{}, false, err
	}
	path, err := memory.NormalizeMemoryPath(path)
	if err != nil {
		return memory.CanonicalMemory{}, false, err
	}
	const query = `SELECT id, tenant, project, namespace, memory_path, class, state, content,
	created_at, updated_at, temporal_fact_id, ingested_at, valid_from, valid_to, validity_source
FROM canonical_memories
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND memory_path = $4 AND class = $5 AND state = $6
ORDER BY updated_at DESC, id DESC LIMIT 1`
	var canonical memory.CanonicalMemory
	err = r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, path, class, memory.MemoryStateActive).Scan(
		&canonical.ID, &canonical.Scope.Tenant, &canonical.Scope.Project, &canonical.Scope.Namespace,
		&canonical.MemoryPath, &canonical.Class, &canonical.State, &canonical.Content,
		&canonical.CreatedAt, &canonical.ModifiedAt, &canonical.TemporalFactID, &canonical.IngestedAt,
		&canonical.ValidFrom, &canonical.ValidTo, &canonical.ValiditySource,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return memory.CanonicalMemory{}, false, nil
	}
	if err != nil {
		return memory.CanonicalMemory{}, false, fmt.Errorf("get latest canonical at exact path: %w", err)
	}
	return canonical, true, nil
}

var _ governance.CanonicalPathRepository = (*Repository)(nil)
