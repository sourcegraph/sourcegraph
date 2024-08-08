use scip::types::{descriptor::Suffix, symbol_information, Descriptor};

pub fn capture_name_to_descriptor(capture: &str, name: String) -> Descriptor {
    Descriptor {
        suffix: match capture {
            "descriptor.method" => Suffix::Method,
            "descriptor.namespace" => Suffix::Namespace,
            "descriptor.type" => Suffix::Type,
            "descriptor.term" => Suffix::Term,

            // TODO: Should consider moving to result here.
            _ => Suffix::UnspecifiedSuffix,
        }
        .into(),
        name,
        ..Default::default()
    }
}

pub fn captures_to_kind(kind: &Option<&str>) -> symbol_information::Kind {
    use symbol_information::Kind::*;

    kind.map_or(UnspecifiedKind, |kind| match kind {
        "kind.accessor" => Accessor,
        "kind.class" => Class,
        "kind.constant" => Constant,
        "kind.constructor" => Constructor,
        "kind.delegate" => Delegate,
        "kind.enum" => Enum,
        "kind.enummember" => EnumMember,
        "kind.event" => Event,
        "kind.field" => Field,
        "kind.function" => Function,
        "kind.getter" => Getter,
        "kind.interface" => Interface,
        "kind.macro" => Macro,
        "kind.method" => Method,
        "kind.methodalias" => MethodAlias,
        "kind.methodspec" => MethodSpecification,
        "kind.module" => Module,
        "kind.namespace" => Namespace,
        "kind.object" => Object,
        "kind.package" => Package,
        "kind.property" => Property,
        "kind.setter" => Setter,
        "kind.singletonmethod" => SingletonMethod,
        "kind.struct" => Struct,
        "kind.typealias" => TypeAlias,
        "kind.trait" => Trait,
        "kind.union" => Union,
        "kind.variable" => Variable,
        // "kind.implementation" => Implementation, TODO
        _ => UnspecifiedKind,
    })
}

// Converts a SCIP symbol kind to the ctags format.
//
// Universal-ctags uses inconsistent kind names for the same concept (for example typealias for
// Kotlin vs. talias for Go). For simplicity, we don't try to match the universal-ctags output, we
// just pass through the SCIP kinds and rely on clients like Zoekt to handle the different naming
// conventions.
//
// When adding a new entry here, be sure to check if Zoekt search ranking needs to be
// updated to recognize the new kind. Eventually we plan to consume SCIP directly in clients, making
// this process less fragile.
pub fn symbol_kind_to_ctags_kind(kind: &symbol_information::Kind) -> Option<&'static str> {
    use symbol_information::Kind::*;

    match kind {
        Accessor => Some("accessor"),
        Class => Some("class"),
        Constant => Some("constant"),
        Constructor => Some("constructor"),
        Delegate => Some("delegate"),
        Enum => Some("enum"),
        EnumMember => Some("enumMember"),
        Event => Some("event"),
        Field => Some("field"),
        Function => Some("function"),
        Getter => Some("getter"),
        Interface => Some("interface"),
        Macro => Some("macro"),
        Method => Some("method"),
        MethodAlias => Some("methodAlias"),
        MethodSpecification => Some("methodSpec"),
        Module => Some("module"),
        Namespace => Some("namespace"),
        Object => Some("object"),
        Package => Some("package"),
        Property => Some("property"),
        Setter => Some("setter"),
        SingletonMethod => Some("singletonMethod"),
        Struct => Some("struct"),
        TypeAlias => Some("typeAlias"),
        Trait => Some("trait"),
        Union => Some("union"),
        Variable => Some("variable"),
        // Implementation => Some("implementation"), TODO
        _ => None,
    }
}
