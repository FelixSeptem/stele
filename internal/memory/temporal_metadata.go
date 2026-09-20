package memory

import "strings"

// TemporalValidityOrigin is the reported provenance class of a validity
// snapshot. It collapses the internal sources into the distinction callers and
// diagnostics actually need: was the interval asserted by a writer, or inferred
// by the service from recorded time?
//
// The distinction matters because an inferred interval is a compatibility
// statement, not a historical claim. Reporting both under one label would let a
// backfilled guess read as evidence.
type TemporalValidityOrigin string

const (
	// TemporalValidityOriginNone means the record carries no interval at all.
	// This is the expected shape for derived artifacts (summaries, relations),
	// which have no fact-valid time of their own.
	TemporalValidityOriginNone TemporalValidityOrigin = "none"
	// TemporalValidityOriginExplicit means a writer asserted the interval.
	TemporalValidityOriginExplicit TemporalValidityOrigin = "explicit"
	// TemporalValidityOriginInferred means the service derived the interval from
	// recorded time (legacy backfill, migration) rather than observing an
	// assertion.
	TemporalValidityOriginInferred TemporalValidityOrigin = "inferred"
)

// TemporalValidityOriginFor classifies a validity source. It is total: an empty
// or unrecognized source classifies as inferred, never as explicit. Failing
// closed in this direction is deliberate -- misreporting an inferred interval as
// explicit is the harmful error, while the reverse is merely conservative.
func TemporalValidityOriginFor(source TemporalValiditySource) TemporalValidityOrigin {
	if source == TemporalValiditySourceExplicit {
		return TemporalValidityOriginExplicit
	}
	return TemporalValidityOriginInferred
}

// TemporalMetadata is the bounded, response-facing view of a validity snapshot.
//
// It reports shape and provenance without re-exposing raw instants: callers
// learn whether the record has an interval, whether that interval is open-ended,
// which source produced it, and whether it is the writer's assertion or the
// service's inference. The underlying recorded/valid timestamps stay in
// privileged reads.
type TemporalMetadata struct {
	Set            bool                   `json:"set"`
	OpenEnded      bool                   `json:"open_ended"`
	Origin         TemporalValidityOrigin `json:"origin"`
	ValiditySource string                 `json:"validity_source,omitempty"`
	Malformed      bool                   `json:"malformed,omitempty"`
}

// NewTemporalMetadata builds the bounded projection for a snapshot.
//
// A malformed snapshot is reported as such and is not presented as a usable
// interval: Set stays false and Origin is withheld. Silently normalizing it
// would be worse than reporting nothing, because a caller could then act on a
// boundary the service never actually validated.
func NewTemporalMetadata(validity TemporalValidity) TemporalMetadata {
	if validity.IsUnset() {
		return TemporalMetadata{Origin: TemporalValidityOriginNone}
	}

	source := TemporalValiditySource(strings.TrimSpace(string(validity.ValiditySource)))
	if err := validity.Validate(); err != nil {
		return TemporalMetadata{
			Origin:         TemporalValidityOriginInferred,
			ValiditySource: string(source),
			Malformed:      true,
		}
	}

	return TemporalMetadata{
		Set:            true,
		OpenEnded:      validity.ValidTo == nil,
		Origin:         TemporalValidityOriginFor(source),
		ValiditySource: string(source),
	}
}

// NewTemporalHistoryMetadata summarizes the temporal shape of a history read.
//
// The current head's metadata is taken from the canonical record; the per-version
// counts then show whether the chain rests on asserted or derived intervals. A
// history that is entirely inferred is still readable, but the caller can see
// that no writer ever asserted the interval -- which is exactly the distinction
// the design requires diagnostics to measure.
func NewTemporalHistoryMetadata(validity TemporalValidity, versions []MemoryVersion, corrections int) TemporalHistoryMetadata {
	metadata := TemporalHistoryMetadata{
		TemporalMetadata: NewTemporalMetadata(validity),
		VersionCount:     len(versions),
		Corrections:      corrections,
	}

	for _, version := range versions {
		switch NewTemporalMetadata(version.TemporalValidity).Origin {
		case TemporalValidityOriginExplicit:
			metadata.ExplicitVersionCount++
		case TemporalValidityOriginInferred:
			metadata.InferredVersionCount++
			metadata.HasInferredValidity = true
		}
	}

	return metadata
}
