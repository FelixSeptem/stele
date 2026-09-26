DROP TABLE IF EXISTS mcp_forget_previews;
DROP TABLE IF EXISTS mcp_forget_apply_operations;
ALTER TABLE access_scope_grants DROP CONSTRAINT IF EXISTS access_scope_grants_access_mode_check;
ALTER TABLE access_scope_grants DROP COLUMN IF EXISTS access_mode;
