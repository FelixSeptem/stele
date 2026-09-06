DROP TRIGGER IF EXISTS context_projection_items_append_only ON context_projection_items;
DROP TRIGGER IF EXISTS context_projections_append_only ON context_projections;
DROP FUNCTION IF EXISTS prevent_context_projection_item_mutation();
DROP FUNCTION IF EXISTS prevent_context_projection_mutation();
DROP TABLE IF EXISTS context_projection_items;
DROP TABLE IF EXISTS context_projections;
