CREATE TABLE IF NOT EXISTS workflows (
    id serial PRIMARY KEY,

    name citext NOT NULL CONSTRAINT workflows_name_max_length CHECK (char_length(name::text) <= 255) CONSTRAINT workflows_name_valid_chars CHECK (name ~ '^[a-zA-Z0-9](?:[a-zA-Z0-9]|[-.](?=[a-zA-Z0-9]))*-?$'::citext),
    description text NOT NULL CONSTRAINT workflows_description_max_length CHECK (char_length(template_text) <= 1024*50),
    template_text text NOT NULL CONSTRAINT workflows_template_text_max_length CHECK (char_length(template_text) <= 1024*100),
    draft boolean NOT NULL DEFAULT false,

    owner_user_id integer REFERENCES users(id) ON DELETE CASCADE,
    owner_org_id integer REFERENCES orgs(id) ON DELETE CASCADE,

    created_by integer NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_by integer NULL REFERENCES users (id) ON DELETE SET NULL,
    updated_at timestamp with time zone NOT NULL DEFAULT now(),

    CONSTRAINT workflows_has_valid_owner CHECK ((((owner_user_id IS NOT NULL) AND (owner_org_id IS NULL)) OR ((owner_org_id IS NOT NULL) AND (owner_user_id IS NULL)))),
    CONSTRAINT workflows_name_is_unique_in_owner UNIQUE (owner_user_id, owner_org_id, name)
);

CREATE OR REPLACE VIEW workflows_view AS
    SELECT
        workflows.*,
        COALESCE(users.username, orgs.name) || '/' || workflows.name AS name_with_owner
        FROM workflows
        LEFT JOIN users ON users.id = workflows.owner_user_id
        LEFT JOIN orgs ON orgs.id = workflows.owner_org_id;
