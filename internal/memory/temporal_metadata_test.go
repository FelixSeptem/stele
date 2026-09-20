package memory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// temporalMetadataTimes returns a coherent set of instants for metadata tests.
func temporalMetadataTimes() (recorded, validFrom time.Time, closed time.Time) {
	recorded = time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	validFrom = time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	closed = time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)
	return
}

func explicitSnapshot(factID string, recorded, validFrom time.Time, validTo *time.Time) TemporalValidity {
	return TemporalValidity{
		TemporalFactID: factID,
		IngestedAt:     recorded,
		ValidFrom:      validFrom,
		ValidTo:        validTo,
		ValiditySource: TemporalValiditySourceExplicit,
	}
}

// TestTemporalValiditySourceClassificationIsTotal proves every declared source
// maps to exactly one of the two reported classes. An unknown or empty source
// must not be silently reported as explicit: that would let an inferred interval
// masquerade as an asserted one.
func TestTemporalValiditySourceClassificationIsTotal(t *testing.T) {
	tests := []struct {
		name   string
		source TemporalValiditySource
		want   TemporalValidityOrigin
	}{
		{
			name:   "explicit",
			source: TemporalValiditySourceExplicit,
			want:   TemporalValidityOriginExplicit,
		},
		{
			name:   "legacy compatible is inferred",
			source: TemporalValiditySourceLegacyCompatible,
			want:   TemporalValidityOriginInferred,
		},
		{
			name:   "migrated is inferred",
			source: TemporalValiditySourceMigrated,
			want:   TemporalValidityOriginInferred,
		},
		{
			name:   "empty source is inferred",
			source: TemporalValiditySource(""),
			want:   TemporalValidityOriginInferred,
		},
		{
			name:   "unknown source is inferred",
			source: TemporalValiditySource("something_else"),
			want:   TemporalValidityOriginInferred,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := TemporalValidityOriginFor(test.source); got != test.want {
				t.Fatalf("TemporalValidityOriginFor(%q) = %q, want %q", test.source, got, test.want)
			}
		})
	}
}

// TestTemporalMetadataCarriesBoundedFieldsOnly proves the projection exposed to
// callers is bounded: it reports whether an identity exists and whether the
// interval is open, but it does not re-expose the raw recorded/valid instants.
func TestTemporalMetadataCarriesBoundedFieldsOnly(t *testing.T) {
	recorded, validFrom, closed := temporalMetadataTimes()

	tests := []struct {
		name           string
		validity       TemporalValidity
		wantSet        bool
		wantOrigin     TemporalValidityOrigin
		wantOpenEnded  bool
		wantSourceName string
	}{
		{
			name:           "explicit open interval",
			validity:       explicitSnapshot("fact-1", recorded, validFrom, nil),
			wantSet:        true,
			wantOrigin:     TemporalValidityOriginExplicit,
			wantOpenEnded:  true,
			wantSourceName: string(TemporalValiditySourceExplicit),
		},
		{
			name:           "explicit closed interval",
			validity:       explicitSnapshot("fact-1", recorded, validFrom, &closed),
			wantSet:        true,
			wantOrigin:     TemporalValidityOriginExplicit,
			wantOpenEnded:  false,
			wantSourceName: string(TemporalValiditySourceExplicit),
		},
		{
			name:           "legacy compatible is inferred",
			validity:       TemporalValidity{}.LegacyCurrentCompatible(recorded),
			wantSet:        true,
			wantOrigin:     TemporalValidityOriginInferred,
			wantOpenEnded:  true,
			wantSourceName: string(TemporalValiditySourceLegacyCompatible),
		},
		{
			name:           "unset stays unset",
			validity:       TemporalValidity{},
			wantSet:        false,
			wantOrigin:     TemporalValidityOriginNone,
			wantOpenEnded:  false,
			wantSourceName: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := NewTemporalMetadata(test.validity)
			if metadata.Set != test.wantSet {
				t.Fatalf("Set = %v, want %v", metadata.Set, test.wantSet)
			}
			if metadata.Origin != test.wantOrigin {
				t.Fatalf("Origin = %q, want %q", metadata.Origin, test.wantOrigin)
			}
			if metadata.OpenEnded != test.wantOpenEnded {
				t.Fatalf("OpenEnded = %v, want %v", metadata.OpenEnded, test.wantOpenEnded)
			}
			if metadata.ValiditySource != test.wantSourceName {
				t.Fatalf("ValiditySource = %q, want %q", metadata.ValiditySource, test.wantSourceName)
			}
		})
	}
}

// TestTemporalMetadataJSONOmitsRawInstants proves the serialized shape cannot
// leak the underlying timestamps. The design commits to keeping raw instants out
// of ordinary responses, so this fails closed on accidental field additions.
func TestTemporalMetadataJSONOmitsRawInstants(t *testing.T) {
	recorded, validFrom, closed := temporalMetadataTimes()
	validity := explicitSnapshot("fact-1", recorded, validFrom, &closed)

	encoded, err := json.Marshal(NewTemporalMetadata(validity))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	payload := string(encoded)

	for _, forbidden := range []string{
		"2026-06-20T08:00:00Z",
		"2026-06-21T09:00:00Z",
		"valid_from",
		"valid_to",
		"ingested_at",
	} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("serialized metadata %s leaks %q", payload, forbidden)
		}
	}

	// The bounded fields must still be present.
	for _, required := range []string{"\"set\"", "\"origin\"", "\"open_ended\"", "\"validity_source\""} {
		if !strings.Contains(payload, required) {
			t.Fatalf("serialized metadata %s is missing %s", payload, required)
		}
	}
}

// TestTemporalMetadataUnsetOmitsSourceName proves an unset snapshot does not
// invent a source name, so a caller cannot mistake "no interval" for an
// inferred interval.
func TestTemporalMetadataUnsetOmitsSourceName(t *testing.T) {
	metadata := NewTemporalMetadata(TemporalValidity{})
	if metadata.Origin == TemporalValidityOriginInferred {
		t.Fatal("unset snapshot reported as inferred; it must report no origin")
	}
	if metadata.ValiditySource != "" {
		t.Fatalf("ValiditySource = %q, want empty", metadata.ValiditySource)
	}
}

// TestTemporalMetadataRejectsMalformedSnapshot proves a malformed snapshot is
// reported as bounded-but-invalid rather than being normalized into a plausible
// looking interval.
func TestTemporalMetadataRejectsMalformedSnapshot(t *testing.T) {
	recorded, validFrom, _ := temporalMetadataTimes()
	closed := validFrom

	malformed := explicitSnapshot("fact-1", recorded, validFrom, &closed)

	metadata := NewTemporalMetadata(malformed)
	if metadata.Set {
		t.Fatal("malformed snapshot reported as a valid set interval")
	}
	if !metadata.Malformed {
		t.Fatal("malformed snapshot did not report Malformed")
	}
	if metadata.Origin == TemporalValidityOriginExplicit {
		t.Fatal("malformed snapshot must not be reported as explicit")
	}
}
