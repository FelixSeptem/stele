-- Source labels and disclosure ceilings cannot be safely erased by a down migration.
-- Disable/revoke affected public credentials before reverting to an older binary.
DO $$ BEGIN RAISE EXCEPTION 'PC2 source metadata/disclosure migration is forward-only; preserve labels and grant history'; END $$;
