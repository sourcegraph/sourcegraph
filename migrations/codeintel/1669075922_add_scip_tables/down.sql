DROP TRIGGER IF EXISTS codeintel_scip_document_lookup_schema_versions_insert ON codeintel_scip_document_lookup_schema_versions;
DROP FUNCTION IF EXISTS update_codeintel_scip_document_lookup_schema_versions_insert;
DROP TABLE IF EXISTS codeintel_scip_document_lookup_schema_versions;

DROP TRIGGER IF EXISTS codeintel_scip_symbols_schema_versions_insert ON codeintel_scip_symbols_schema_versions;
DROP FUNCTION IF EXISTS update_codeintel_scip_symbols_schema_versions_insert;
DROP TABLE IF EXISTS codeintel_scip_symbols_schema_versions;

DROP TABLE IF EXISTS codeintel_scip_symbols;
DROP TABLE IF EXISTS codeintel_scip_document_lookup;
DROP TABLE IF EXISTS codeintel_scip_documents;
DROP TABLE IF EXISTS codeintel_scip_metadata;
