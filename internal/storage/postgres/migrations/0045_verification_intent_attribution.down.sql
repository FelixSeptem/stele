-- Production-safe rollback retains verification-to-intent audit links.
-- Disable verification correction admission before binary rollback; deleting
-- these rows would erase governed correction provenance.
DO $$ BEGIN
    RAISE EXCEPTION 'verification intent attribution downgrade is unsupported; disable capability and retain the ledger';
END $$;
