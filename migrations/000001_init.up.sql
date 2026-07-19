CREATE TYPE record_type AS ENUM (
    'decision',
    'failed_attempt',
    'constraint',
    'discovery',
    'open_question'
);

CREATE TYPE record_status AS ENUM (
    'draft',
    'active',
    'superseded',
    'archived',
    'invalid'
);

CREATE TYPE relationship_type AS ENUM (
    'supersedes',
    'relates_to',
    'contradicts',
    'depends_on',
    'duplicates'
);

CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE projects (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    root_path  text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER projects_set_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE records (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    session_id      uuid,
    record_type     record_type NOT NULL,
    title           text NOT NULL,
    task            text NOT NULL,
    summary         text NOT NULL,
    chosen_approach text,
    rationale       text,
    status          record_status NOT NULL DEFAULT 'draft',
    confidence      real CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    created_by      text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    search_vector   tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(summary, '')), 'B') ||
        setweight(to_tsvector('english', coalesce(rationale, '')), 'C')
    ) STORED
);

CREATE INDEX records_search_vector_idx ON records USING GIN (search_vector);
CREATE INDEX records_project_status_idx ON records (project_id, status);
CREATE INDEX records_session_idx ON records (session_id);

CREATE TRIGGER records_set_updated_at
    BEFORE UPDATE ON records
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE alternatives (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id uuid NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    approach  text NOT NULL,
    result    text,
    reason    text,
    position  int NOT NULL DEFAULT 0
);

CREATE INDEX alternatives_record_idx ON alternatives (record_id);

CREATE TABLE record_files (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id   uuid NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    file_path   text NOT NULL,
    commit_hash text,
    UNIQUE (record_id, file_path)
);

CREATE INDEX record_files_record_idx ON record_files (record_id);
CREATE INDEX record_files_path_idx ON record_files (file_path);

CREATE TABLE relationships (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id         uuid NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    target_record_id  uuid NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    relationship_type relationship_type NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (record_id, target_record_id, relationship_type),
    CHECK (record_id <> target_record_id)
);

CREATE INDEX relationships_target_idx ON relationships (target_record_id);
