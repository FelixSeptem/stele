package postgres

import (
	"encoding/json"
	"fmt"
	"github.com/FelixSeptem/stele/internal/memory"
	"strings"
)

func insightSourcesExpression(refs []memory.DerivedInsightEvidenceRef, scope memory.Scope) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		predicate := " AND tenant=" + sourceSQLString(scope.Tenant) + " AND project=" + sourceSQLString(scope.Project) + " AND namespace=" + sourceSQLString(scope.Namespace)
		switch ref.Kind {
		case memory.DerivedInsightEvidenceKindRawEvent:
			parts = append(parts, "(SELECT source_metadata FROM raw_events WHERE id::text="+sourceSQLString(ref.ID)+predicate+")")
		case memory.DerivedInsightEvidenceKindCanonicalMemory, memory.DerivedInsightEvidenceKindProceduralMemory, memory.DerivedInsightEvidenceKindSummaryMemory, memory.DerivedInsightEvidenceKindRelationMemory:
			parts = append(parts, "(SELECT source_metadata FROM canonical_memories WHERE id::text="+sourceSQLString(ref.ID)+predicate+")")
		default:
			parts = append(parts, "NULL::jsonb")
		}
	}
	if len(parts) == 0 {
		return sourceMetadataLiteral(memory.SourceMetadata{})
	}
	return "stele_source_join(ARRAY[" + strings.Join(parts, ",") + "])"
}

// Only validated fixed enums enter this SQL literal, never user strings. Keeping
// these values in the statement lets legacy unlabeled writes use the same path.
func sourceMetadataLiteral(value memory.SourceMetadata) string {
	raw, _ := json.Marshal(value.Effective())
	return "'" + string(raw) + "'::jsonb"
}

// Add eligibility to the canonical query itself, before ordering and LIMIT.
// Source versions are read in the same statement as the selected snapshot.
func disclosureSearchQuery(query, alias string) string {
	prefix := "canonical_memories."
	if alias != "" {
		prefix = alias + "."
	}
	from := strings.Index(query, "\nFROM canonical_memories")
	if alias == "cm" && strings.Contains(query, "FROM relation_projections") {
		from = strings.Index(query, "\nFROM relation_projections")
	}
	if from < 0 {
		panic("disclosure search requires canonical source")
	}
	fields := fmt.Sprintf(",\n\t%ssource_metadata, (SELECT COALESCE(MAX(sv.version),0) FROM memory_versions sv WHERE sv.memory_id=%sid) AS source_version", prefix, prefix)
	query = query[:from] + fields + query[from:]
	order := strings.LastIndex(query, "\nORDER BY")
	predicate := fmt.Sprintf("\n\tAND %sstate='active' AND %ssource_metadata->>'version'='source-metadata-v1' AND %ssource_metadata->>'trust' IN ('untrusted','attested_data') AND %ssource_metadata->>'classification' IN ('low','medium')", prefix, prefix, prefix, prefix)
	// A summary's own labels cannot make currently hidden source content visible.
	// Validate all durable raw-event edges against their independently eligible
	// canonical sources before ranking and LIMIT. Unverifiable summaries are
	// omitted; an authorization dependency error still fails the SQL operation.
	predicate += fmt.Sprintf(`
	AND (%[1]sclass <> 'summary' OR (
		EXISTS (SELECT 1 FROM provenance_links p WHERE p.memory_id=%[1]sid AND p.operation='create_summary_memory')
		AND NOT EXISTS (
			SELECT 1 FROM provenance_links p
			LEFT JOIN raw_events e ON e.id=p.raw_event_id
			WHERE p.memory_id=%[1]sid AND p.operation='create_summary_memory' AND (
				p.tenant<>%[1]stenant OR p.project<>%[1]sproject OR p.namespace<>%[1]snamespace
				OR e.id IS NULL OR e.tenant<>%[1]stenant OR e.project<>%[1]sproject OR e.namespace<>%[1]snamespace
				OR e.source_metadata->>'classification' NOT IN ('low','medium')
				OR NOT EXISTS (
					SELECT 1 FROM provenance_links edge JOIN canonical_memories src ON src.id=edge.memory_id
					WHERE edge.raw_event_id=p.raw_event_id AND edge.operation='promote_candidate'
					AND edge.tenant=%[1]stenant AND edge.project=%[1]sproject AND edge.namespace=%[1]snamespace
					AND src.tenant=%[1]stenant AND src.project=%[1]sproject AND src.namespace=%[1]snamespace
					AND src.state='active' AND src.source_metadata->>'version'='source-metadata-v1'
					AND src.source_metadata->>'trust' IN ('untrusted','attested_data')
					AND src.source_metadata->>'classification' IN ('low','medium')
					AND EXISTS (SELECT 1 FROM memory_versions v WHERE v.memory_id=src.id)
				)
			)
		)
	))`, prefix)
	return query[:order] + predicate + query[order:]
}

// sourceSnapshotInsert adds a durable label expression to an existing INSERT
// while preserving its parameter positions and RETURNING projection.
func sourceSnapshotInsert(query, expression string) string {
	values := strings.Index(query, "VALUES (")
	if values < 0 {
		panic("source snapshot requires INSERT VALUES")
	}
	end := strings.LastIndex(query[:values], ")")
	if end < 0 {
		panic("source snapshot requires INSERT VALUES")
	}
	query = query[:end] + ", source_metadata" + query[end:]
	returning := strings.Index(query, "RETURNING")
	if returning < 0 {
		panic("source snapshot requires RETURNING")
	}
	valuesEnd := returning
	if conflict := strings.Index(query, "ON CONFLICT"); conflict >= 0 && conflict < valuesEnd {
		valuesEnd = conflict
	}
	close := strings.LastIndex(query[:valuesEnd], ")")
	return query[:close] + ", " + expression + query[close:]
}

func sourceSQLString(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

// Explicit marker for QueryRow projections, which do not expose column metadata.
type sourceSnapshotRow struct {
	ScanRow interface{ Scan(...any) error }
}

func (r sourceSnapshotRow) Scan(dest ...any) error { return r.ScanRow.Scan(dest...) }
func sourceReturning(query string) string          { return strings.TrimSpace(query) + ", source_metadata" }

func candidateSourceExpression(id string, scope memory.Scope) string {
	return "stele_source_join(ARRAY[(SELECT source_metadata FROM candidate_memories WHERE id=" + sourceSQLString(id) + " AND tenant=" + sourceSQLString(scope.Tenant) + " AND project=" + sourceSQLString(scope.Project) + " AND namespace=" + sourceSQLString(scope.Namespace) + ")])"
}

func rawSourcesExpression(ids []string, scope memory.Scope) string {
	expressions := make([]string, 0, len(ids))
	for _, id := range ids {
		expressions = append(expressions, "(SELECT source_metadata FROM raw_events WHERE id="+sourceSQLString(id)+" AND tenant="+sourceSQLString(scope.Tenant)+" AND project="+sourceSQLString(scope.Project)+" AND namespace="+sourceSQLString(scope.Namespace)+")")
		// Summary compaction consumes extracted candidates. Preserve their
		// sensitivity floors as well as the original event attribution.
		expressions = append(expressions, "(SELECT stele_source_join(array_agg(source_metadata)) FROM candidate_memories WHERE source_raw_event_id="+sourceSQLString(id)+" AND tenant="+sourceSQLString(scope.Tenant)+" AND project="+sourceSQLString(scope.Project)+" AND namespace="+sourceSQLString(scope.Namespace)+")")
	}
	if len(expressions) == 0 {
		return sourceMetadataLiteral(memory.SourceMetadata{})
	}
	return "stele_source_join(ARRAY[" + strings.Join(expressions, ",") + "])"
}
