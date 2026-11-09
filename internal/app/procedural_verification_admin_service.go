package app

import (
	"context"

	"github.com/FelixSeptem/stele/internal/memory"
)

// proceduralVerificationAdminService composes the narrow domain services at
// the privileged HTTP boundary. It has no Provider or ordinary retrieval
// dependency, keeping verification evidence out of public runtime contracts.
type proceduralVerificationAdminService struct {
	verification memory.ProceduralVerificationService
	revalidation memory.ProceduralRevalidationService
	correction   memory.VerificationCorrectionAdmissionService
	reader       proceduralVerificationAdminReader
}

type proceduralVerificationAdminReader interface {
	ReadProceduralMemoryVerification(context.Context, memory.Scope, string) (memory.ProceduralMemoryVerification, error)
	ListProceduralMemoryVerificationHistory(context.Context, memory.Scope, string, int64, int) ([]memory.ProceduralMemoryVerification, error)
	ReadProceduralVerificationLineage(context.Context, memory.Scope, string, int64) (memory.ProceduralVerificationLineage, error)
	ReadProceduralRevalidationRun(context.Context, memory.Scope, string) (memory.ProceduralRevalidationRun, error)
}

func (s proceduralVerificationAdminService) Record(ctx context.Context, record memory.ProceduralMemoryVerification) (memory.ProceduralMemoryVerification, error) {
	return s.verification.Record(ctx, record)
}

func (s proceduralVerificationAdminService) ReadProceduralMemoryVerification(ctx context.Context, scope memory.Scope, id string) (memory.ProceduralMemoryVerification, error) {
	return s.reader.ReadProceduralMemoryVerification(ctx, scope, id)
}

func (s proceduralVerificationAdminService) ListProceduralMemoryVerificationHistory(ctx context.Context, scope memory.Scope, memoryID string, targetVersion int64, limit int) ([]memory.ProceduralMemoryVerification, error) {
	return s.reader.ListProceduralMemoryVerificationHistory(ctx, scope, memoryID, targetVersion, limit)
}

func (s proceduralVerificationAdminService) ReadProceduralVerificationLineage(ctx context.Context, scope memory.Scope, memoryID string, targetVersion int64) (memory.ProceduralVerificationLineage, error) {
	return s.reader.ReadProceduralVerificationLineage(ctx, scope, memoryID, targetVersion)
}

func (s proceduralVerificationAdminService) ReadProceduralRevalidationRun(ctx context.Context, scope memory.Scope, runID string) (memory.ProceduralRevalidationRun, error) {
	return s.reader.ReadProceduralRevalidationRun(ctx, scope, runID)
}

func (s proceduralVerificationAdminService) Request(ctx context.Context, request memory.ProceduralRevalidationRequest) (memory.ProceduralRevalidationRun, error) {
	return s.revalidation.Request(ctx, request)
}

func (s proceduralVerificationAdminService) Submit(ctx context.Context, request memory.VerificationCorrectionRequest) (memory.MemoryIntentRecord, error) {
	return s.correction.Submit(ctx, request)
}
