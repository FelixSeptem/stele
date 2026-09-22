CREATE INDEX IF NOT EXISTS relation_projections_graph_source_scope_idx
    ON relation_projections (tenant, project, namespace, source_entity, updated_at DESC);

CREATE INDEX IF NOT EXISTS relation_projections_graph_target_scope_idx
    ON relation_projections (tenant, project, namespace, target_entity, updated_at DESC);

CREATE INDEX IF NOT EXISTS relation_projections_graph_source_currency_idx
    ON relation_projections (tenant, project, namespace, source_version, updated_at DESC);
