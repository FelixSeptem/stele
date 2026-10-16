-- Evidence is retained on operational rollback. Disable the PC6 route/capability
-- instead; destructive downgrade would discard accepted audit history.
DO $$ BEGIN RAISE EXCEPTION 'PC6 evidence downgrade is unsupported; disable the capability and retain the ledger'; END $$;
