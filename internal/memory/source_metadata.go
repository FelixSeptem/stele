package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const SourceMetadataVersion = "source-metadata-v1"

type SourceTrust string

const (
	SourceTrustUntrusted SourceTrust = "untrusted"
	// Attestation verifies authorized source attribution, never instruction authority.
	SourceTrustAttestedData SourceTrust = "attested_data"
)

type SourceClassification string

const (
	SourceClassificationLow        SourceClassification = "low"
	SourceClassificationMedium     SourceClassification = "medium"
	SourceClassificationHigh       SourceClassification = "high"
	SourceClassificationRestricted SourceClassification = "restricted"
)

type SourceMetadata struct {
	Version        string               `json:"version"`
	Trust          SourceTrust          `json:"trust"`
	Classification SourceClassification `json:"classification"`
}

func (m SourceMetadata) MarshalJSON() ([]byte, error) {
	type snapshot SourceMetadata
	return json.Marshal(snapshot(m.Effective()))
}

func (c SourceClassification) rank() int {
	switch c {
	case SourceClassificationLow:
		return 0
	case SourceClassificationMedium:
		return 1
	case SourceClassificationHigh:
		return 2
	case SourceClassificationRestricted:
		return 3
	default:
		return -1
	}
}

func (m SourceMetadata) Validate() error {
	if m.Version != SourceMetadataVersion || (m.Trust != SourceTrustUntrusted && m.Trust != SourceTrustAttestedData) || m.Classification.rank() < 0 {
		return fmt.Errorf("invalid source metadata")
	}
	return nil
}

// Effective fails closed for legacy and malformed persisted snapshots.
func (m SourceMetadata) Effective() SourceMetadata {
	if m.Validate() != nil {
		return SourceMetadata{Version: SourceMetadataVersion, Trust: SourceTrustUntrusted, Classification: SourceClassificationRestricted}
	}
	return m
}

func (m SourceMetadata) Disclosable() bool {
	m = m.Effective()
	return m.Classification == SourceClassificationLow || m.Classification == SourceClassificationMedium
}

// JoinSourceMetadata is shared by every derivation; incomplete lineage is restricted.
func JoinSourceMetadata(sources []SourceMetadata, complete bool, floor SourceClassification) SourceMetadata {
	if !complete || len(sources) == 0 || floor.rank() < 0 {
		return (SourceMetadata{}).Effective()
	}
	result := SourceMetadata{Version: SourceMetadataVersion, Trust: SourceTrustAttestedData, Classification: floor}
	for _, source := range sources {
		source = source.Effective()
		if source.Trust != SourceTrustAttestedData {
			result.Trust = SourceTrustUntrusted
		}
		if source.Classification.rank() > result.Classification.rank() {
			result.Classification = source.Classification
		}
	}
	return result
}

// UnmarshalJSON uses canonical fields and rejects duplicates, aliases and nulls.
func (m *SourceMetadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("source metadata object required")
	}
	values := map[string]string{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return fmt.Errorf("invalid source metadata")
		}
		key, ok := token.(string)
		if !ok || (key != "version" && key != "trust" && key != "classification") {
			return fmt.Errorf("invalid source metadata field")
		}
		if _, duplicate := values[key]; duplicate {
			return fmt.Errorf("duplicate source metadata field")
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("invalid source metadata value")
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("invalid source metadata value")
		}
		values[key] = value
	}
	if _, err := d.Token(); err != nil {
		return fmt.Errorf("invalid source metadata")
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("invalid source metadata")
	}
	result := SourceMetadata{Version: values["version"], Trust: SourceTrust(values["trust"]), Classification: SourceClassification(values["classification"])}
	if err := result.Validate(); err != nil {
		return err
	}
	*m = result
	return nil
}
