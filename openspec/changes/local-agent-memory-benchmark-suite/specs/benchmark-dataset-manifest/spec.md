## ADDED Requirements

### Requirement: Dataset manifest is versioned and auditable
The benchmark system SHALL represent every dataset release with a manifest containing name, version, license, upstream URL, upstream commit or tag, SHA256, source path, conversion version, available splits, redistribution status, and embedding profile.

#### Scenario: Valid manifest is accepted
- **WHEN** a manifest contains all required fields and a supported schema version
- **THEN** the system accepts it and exposes a stable dataset identity of name, version, and checksum

#### Scenario: Missing provenance is rejected
- **WHEN** a manifest omits license, upstream revision, or SHA256
- **THEN** validation fails with `invalid_manifest` and identifies the missing field

### Requirement: Fetch writes content-addressed cache metadata
The fetch operation SHALL store raw data under the configured dataset/version cache and SHALL verify the declared SHA256 before replacing or creating a cache lock.

#### Scenario: Checksum verification succeeds
- **WHEN** downloaded bytes match the manifest SHA256
- **THEN** the cache lock records the checksum, upstream revision, and fetch timestamp

#### Scenario: Checksum mismatch is non-destructive
- **WHEN** downloaded bytes do not match the manifest SHA256
- **THEN** fetch returns `checksum_mismatch` and leaves any existing valid cache untouched

### Requirement: Redistribution restrictions are explicit
The repository SHALL store only manifest metadata, conversion code, documentation, and explicitly permitted smoke fixtures for datasets whose redistribution status is restricted or unknown.

#### Scenario: Restricted dataset is prepared locally
- **WHEN** a manifest marks full data as non-redistributable
- **THEN** fetch instructions require a user-provided local source and repository checks do not expect the full archive to exist in Git

### Requirement: Cache paths and splits are deterministic
The system SHALL use `<data-dir>/<dataset>/<version>/{raw,normalized,embeddings,reports}` and SHALL distinguish smoke and full splits in metadata and run inputs.

#### Scenario: Same inputs resolve to same cache
- **WHEN** two fetch or run invocations use identical data directory, dataset, and version
- **THEN** they resolve to the same cache root and split identifiers
