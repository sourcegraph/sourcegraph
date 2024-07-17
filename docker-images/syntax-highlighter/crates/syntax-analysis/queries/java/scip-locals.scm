(class_declaration) @scope
(interface_declaration) @scope
(enum_declaration) @scope
(record_declaration) @scope
(method_declaration) @scope
(constructor_declaration) @scope
(lambda_expression) @scope
(enhanced_for_statement) @scope
(for_statement) @scope
(block) @scope


; NOTE: The definitions below are commented out
; as they overlap with global symbol indexing
; marking type declarations as locals causes
; various confusions, for example around constructors
;
; They are kept here for reference and to avoid re-introducing them

; (class_declaration
;     name: (identifier) @definition.type
; )

; (interface_declaration
;     name: (identifier) @definition.type
; )

; (enum_declaration
;     name: (identifier) @definition.type
; )

; (record_declaration
;     name: (identifier) @definition.type
; )

; (method_declaration
;     name: (identifier) @definition.function
; )

; (enum_constant
;   name: (identifier) @definition.term
; )

(enhanced_for_statement
    name: (identifier) @definition.term)

(lambda_expression

  parameters: [
   (identifier) @definition.term

   (inferred_parameters
        (identifier) @definition.term
    )
  ]
)

(record_declaration
 (formal_parameters
  (formal_parameter
   name: (identifier) @occurrence.skip)))

(formal_parameter
    name: (identifier) @definition.term
)

(field_declaration
 (variable_declarator
  name: (identifier) @occurrence.skip))

(variable_declarator
    name: (identifier) @definition.term
)

(record_pattern_component
  (identifier) @definition.term
)

; REFERENCES

; import java.util.HashSet
;        ^^^^^^^^^ namespace
;                  ^^^^^^^ type (could also be a constant, but type is more common)
(import_declaration
  (scoped_identifier
    scope: (_) @reference.namespace
    name: (_) @reference.type))

(field_access object: (identifier) @reference)
(field_access field: (identifier) @reference.global.term)

; hello(...)
; ^^^^^
; As we don't support local methods yet, we unequivocally mark this reference
; as global
(method_invocation
  name: (identifier) @reference.global.method
)

; MyType variable = ...
; ^^^^^^
(local_variable_declaration
  type: (type_identifier) @occurrence.skip
    (#eq? @reference.type "var")
)

; class Binary<N extends Number> {...
;                        ^^^^^^
(type_bound
  (type_identifier)* @reference.type
)


; Person::getName
; ^^^^^^  ^^^^^^^
(method_reference (identifier)* @reference.global.method)

; type references are generally global
(type_identifier) @reference.type

; all other references we assume to be local only
(identifier) @reference.local
