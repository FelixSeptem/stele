## ADDED Requirements

### Requirement: Task evaluations may cite procedural verification evidence
The service SHALL allow an authorized task evaluation to link one same-scope
procedural-memory verification as evidence. The link MUST preserve verification
identity and disposition for authorized audit without copying claim content into
public report fields or granting the task verdict authority to mutate memory or
verification state.

#### Scenario: Externally judged task cites a verification
- **WHEN** an authorized caller records a task evaluation with an eligible
  same-scope procedural verification reference
- **THEN** the service stores the bounded evidence link and retains the external
  verdict as externally supplied task evidence

#### Scenario: Task evaluation cites an ineligible verification
- **WHEN** a task evaluation names a foreign, hidden, unavailable, or
  lifecycle-ineligible verification
- **THEN** the service rejects or omits the link without revealing verification
  or target-memory existence
