-- Production-safe rollback retains accepted correction-to-intent audit links.
-- Disable the Provider correction capability in application configuration before
-- a binary rollback; destructive removal would erase governed provenance.
DO $$ BEGIN
    RAISE EXCEPTION 'provider correction attribution downgrade is unsupported; disable capability and retain the ledger';
END $$;
