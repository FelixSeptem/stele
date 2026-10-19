-- Turn ownership and attribution are security evidence. Disable the capability
-- and retain this history; destructive downgrade is intentionally unsupported.
DO $$ BEGIN RAISE EXCEPTION 'Provider context turn-binding downgrade is unsupported; disable capability and retain ownership/audit history'; END $$;
