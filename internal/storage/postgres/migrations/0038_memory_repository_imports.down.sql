DO $$ BEGIN RAISE EXCEPTION 'memory repository import downgrade is unsupported; disable admission and retain audit history'; END $$;
