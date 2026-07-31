-- Optional code repository URL for each service direction (empty = no link shown).
ALTER TABLE als_service_directions ADD COLUMN repo_url TEXT NOT NULL DEFAULT '';
