CREATE TABLE IF NOT EXISTS codeintel_scip_symbols_lookup (
    upload_id       integer                  NOT NULL,
    segment_type    SymbolNameSegmentType    NOT NULL,
    segment_quality SymbolNameSegmentQuality,
    name            text                     NOT NULL,
    id              integer                  NOT NULL,
    parent_id       integer
);

-- Reconstruct names from leaf (descriptor) in tree
CREATE UNIQUE INDEX IF NOT EXISTS codeintel_scip_symbols_lookup_id ON codeintel_scip_symbols_lookup(upload_id, id);

-- Search by descriptor suffix (supports fast exact + suffix match by comparing reversed string or wildcard)
CREATE INDEX IF NOT EXISTS codeintel_scip_symbols_lookup_reversed_descriptor_suffix_name
    ON codeintel_scip_symbols_lookup(upload_id, reverse(name) text_pattern_ops)
    WHERE segment_type = 'DESCRIPTOR_SUFFIX';

CREATE TABLE IF NOT EXISTS codeintel_scip_symbols_lookup_leaves (
    upload_id                   integer NOT NULL,
    symbol_id                   integer NOT NULL,
    descriptor_suffix_id        integer NOT NULL,
    fuzzy_descriptor_suffix_id  integer
);

CREATE INDEX IF NOT EXISTS codeintel_scip_symbols_lookup_leaves_descriptor_suffix_id ON codeintel_scip_symbols_lookup_leaves(upload_id, descriptor_suffix_id);
CREATE INDEX IF NOT EXISTS codeintel_scip_symbols_lookup_leaves_fuzzy_descriptor_suffix_id ON codeintel_scip_symbols_lookup_leaves(upload_id, fuzzy_descriptor_suffix_id) WHERE fuzzy_descriptor_suffix_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS codeintel_scip_symbols_migration_progress (
    upload_id  integer NOT NULL PRIMARY KEY,
    symbol_id  integer NOT NULL
);
