(source_file
    (package (identifier) @descriptor.namespace @kind.package)
) @scope

; This matches global top-level assignments
(fragment "_global" "_constant" (identifier) @kind.constant @descriptor.term)
(fragment "_global" "_constant"? @cons
    (identifier) @kind.variable @descriptor.term
    (#not-eq? @cons "_constant"))

(invoke
    receiver: (variable) @name
    (symbol) @descriptor.type @kind.class
    (#eq? @name "def_slotted_exemplar")
    (#transform! ":(.*)" "$1"))

(invoke
    receiver: (variable) @name
    (symbol) @descriptor.type @kind.class
    (#eq? @name "def_mixin")
    (#transform! ":(.*)" "$1"))

(method
    exemplarname: (_) @descriptor.type
    name: (_) @descriptor.method @kind.method)

[
    (procedure)
    (block)
    (iterator)
    (while)
    (try)
    (loop)
    (if)
] @local
