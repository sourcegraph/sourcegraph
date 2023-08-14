DROP TYPE IF EXISTS SymbolNameSegmentType;

CREATE TYPE SymbolNameSegmentType AS ENUM (
    'SCHEME',
    'PACKAGE_MANAGER',
    'PACKAGE_NAME',
    'PACKAGE_VERSION',
    'DESCRIPTOR_NAMESPACE',
    'DESCRIPTOR_SUFFIX'
);

DROP TYPE IF EXISTS SymbolNameSegmentQuality;

CREATE TYPE SymbolNameSegmentQuality AS ENUM (
    'FUZZY',
    'PRECISE',
    'BOTH'
);
