package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestVerificationCorrectionAdmissionSubmitsOnlyCurrentRefutedProcedure(t *testing.T) {
	verification := serviceProceduralVerification()
	verification.Disposition = ProceduralVerificationDispositionRefuted
	verification.CorrectedContent = "wait for confirmation before retrying"
	reader := &verificationCorrectionReaderStub{verification: verification}
	submitter := &verificationCorrectionSubmitterStub{}
	service := VerificationCorrectionAdmissionService{
		Reader: reader, Targets: &proceduralVerificationTargetResolverStub{}, Evidence: &proceduralVerificationEvidenceResolverStub{},
		Policy: StaticProceduralVerificationPolicy{Scope: verification.Scope, Enabled: true, Version: verification.PolicyVersion}, Submitter: submitter,
	}
	got, err := service.Submit(context.Background(), VerificationCorrectionRequest{
		Scope: verification.Scope, VerificationID: verification.ID, Actor: "admin-1", Reason: "observed correction", RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetMemoryID != verification.MemoryID || got.TargetVersion != verification.TargetVersion || got.Type != MemoryIntentUpdate || got.Content != verification.CorrectedContent {
		t.Fatalf("governed update = %+v", got)
	}
	if len(submitter.inputs) != 1 || submitter.attributions[0].VerificationID != verification.ID {
		t.Fatalf("submission inputs=%+v attributions=%+v", submitter.inputs, submitter.attributions)
	}
}

func TestVerificationCorrectionAdmissionRejectsStaleTargetBeforeSubmit(t *testing.T) {
	verification := serviceProceduralVerification()
	verification.Disposition = ProceduralVerificationDispositionRefuted
	verification.CorrectedContent = "wait for confirmation before retrying"
	submitter := &verificationCorrectionSubmitterStub{}
	service := VerificationCorrectionAdmissionService{
		Reader: &verificationCorrectionReaderStub{verification: verification}, Targets: &proceduralVerificationTargetResolverStub{err: errors.New("target stale")}, Evidence: &proceduralVerificationEvidenceResolverStub{},
		Policy: StaticProceduralVerificationPolicy{Scope: verification.Scope, Enabled: true, Version: verification.PolicyVersion}, Submitter: submitter,
	}
	_, err := service.Submit(context.Background(), VerificationCorrectionRequest{Scope: verification.Scope, VerificationID: verification.ID, Actor: "admin-1", Reason: "observed correction", RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "key-1"})
	if err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("Submit() error = %v", err)
	}
	if len(submitter.inputs) != 0 {
		t.Fatalf("stale verification created governed intent: %+v", submitter.inputs)
	}
}

func TestVerificationCorrectionAdmissionRefusesForeignHiddenAndDisabledSourcesBeforeSubmit(t *testing.T) {
	base := serviceProceduralVerification()
	base.Disposition = ProceduralVerificationDispositionRefuted
	base.CorrectedContent = "wait for confirmation before retrying"
	request := VerificationCorrectionRequest{Scope: base.Scope, VerificationID: base.ID, Actor: "admin-1", Reason: "observed correction", RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "key-1"}
	for _, test := range []struct {
		name    string
		service VerificationCorrectionAdmissionService
	}{
		{
			name: "foreign", service: VerificationCorrectionAdmissionService{
				Reader: &verificationCorrectionReaderStub{verification: func() ProceduralMemoryVerification {
					foreign := base
					foreign.Scope.Tenant = "tenant-b"
					return foreign
				}()},
				Targets: &proceduralVerificationTargetResolverStub{}, Evidence: &proceduralVerificationEvidenceResolverStub{}, Policy: enabledProceduralVerificationPolicy(base.Scope), Submitter: &verificationCorrectionSubmitterStub{},
			},
		},
		{
			name: "hidden", service: VerificationCorrectionAdmissionService{
				Reader: &verificationCorrectionReaderStub{err: ErrNotFound}, Targets: &proceduralVerificationTargetResolverStub{}, Evidence: &proceduralVerificationEvidenceResolverStub{}, Policy: enabledProceduralVerificationPolicy(base.Scope), Submitter: &verificationCorrectionSubmitterStub{},
			},
		},
		{
			name: "policy disabled", service: VerificationCorrectionAdmissionService{
				Reader: &verificationCorrectionReaderStub{verification: base}, Targets: &proceduralVerificationTargetResolverStub{}, Evidence: &proceduralVerificationEvidenceResolverStub{},
				Policy: StaticProceduralVerificationPolicy{Scope: base.Scope, Enabled: false, Version: base.PolicyVersion}, Submitter: &verificationCorrectionSubmitterStub{},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.service.Submit(context.Background(), request)
			if err == nil {
				t.Fatal("Submit() accepted an ineligible correction source")
			}
			submitter := test.service.Submitter.(*verificationCorrectionSubmitterStub)
			if len(submitter.inputs) != 0 {
				t.Fatalf("ineligible correction submitted governed input: %+v", submitter.inputs)
			}
		})
	}
}

type verificationCorrectionReaderStub struct {
	verification ProceduralMemoryVerification
	err          error
}

func (s *verificationCorrectionReaderStub) ReadProceduralMemoryVerification(context.Context, Scope, string) (ProceduralMemoryVerification, error) {
	return s.verification, s.err
}

type verificationCorrectionSubmitterStub struct {
	inputs       []MemoryIntentInput
	attributions []VerificationAttribution
}

func (s *verificationCorrectionSubmitterStub) SubmitVerificationAttributedIntent(_ context.Context, input MemoryIntentInput, attribution VerificationAttribution) (MemoryIntentRecord, error) {
	s.inputs = append(s.inputs, input)
	s.attributions = append(s.attributions, attribution)
	return MemoryIntentRecord{ID: "intent-1", Scope: input.Scope, Type: input.Type, TargetMemoryID: input.TargetMemoryID, TargetVersion: input.TargetVersion, Content: input.Content}, nil
}
