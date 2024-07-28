; Based on https://github.com/nvim-treesitter/nvim-treesitter/blob/master/queries/hack/highlights.scm
; To test the syntax of a Hack file you can run the compiler with the following steps
; > docker pull hhvm/hhvm
; > docker run  -v /local/path:/mount/path --tty --interactive hhvm/hhvm:latest /bin/bash -l
; > hhvm --version

((variable) @variable.builtin
  (#eq? @variable.builtin "$this"))

[
  (comment)
  (xhp_comment)
] @comment

(scope_identifier) @keyword
(visibility_modifier) @keyword

(xhp_open
  [
    "<"
    ">"
  ] @tag.delimiter)

(xhp_close
  [
    "</"
    ">"
  ] @tag.delimiter)


(xhp_open_close
  [
    "<"
    "/>"
  ] @tag.delimiter)

[
  (abstract_modifier)
  (final_modifier)
  (static_modifier)
  (visibility_modifier)
  (xhp_modifier)
  (inout_modifier)
  (reify_modifier)
  "?as"
  "as"
  "as"
  "async"
  "attribute"
  "await"
  "break"
  "case"
  "catch"
  "class"
  "clone"
  "concurrent"
  "const"
  "continue"
  "do"
  "echo"
  "else"
  "elseif"
  "enum"
  "extends"
  "finally"
  "for"
  "foreach"
  "function"
  "if"
  "implements"
  "include_once"
  "include"
  "insteadof"
  "interface"
  "is"
  "list"
  ; "nameof" This is missing in the grammar
  "namespace"
  "new"
  "newtype"
  "print"
  "require_once"
  "require"
  "return"
  "super"
  "switch"
  "throw"
  "trait"
  "try"
  "type"
  "use"
  "using"
  "where"
  "while"
  "yield"
] @keyword

(new_expression
  . (_) @type)

(new_expression
  (qualified_identifier
    (identifier) @type))

(scoped_identifier
  (qualified_identifier
    (identifier) @type))

(xhp_class_attribute
    name: (xhp_identifier) @variable
)

[
  (array_type)
  "arraykey"
  "bool"
  "dynamic"
  "float"
  "int"
  "mixed"
  "nonnull"
  "noreturn"
  "nothing"
  "num"
  "shape" ; also a keyword, but prefer highlighting as a type
  "string"
  "tuple" ; also a keyword, but prefer highlighting as a type
  "void"
] @type.builtin

(shape_type_specifier (open_modifier)) @type.builtin

(field_specifier (optional_modifier) @identifier.operator)

(null) @constant.null

[(true) (false)] @boolean

[
  (nullable_modifier)
  (soft_modifier)
  (like_modifier)
] @operator

(alias_declaration
  ["newtype" "type"]
  .
  (identifier) @type)

(class_declaration
  name: (identifier) @type)

(interface_declaration
  name: (identifier) @type)

(type_parameter
  name: (identifier) @type)

(collection
  (qualified_identifier
    (identifier) @type .))

(attribute_modifier (qualified_identifier (identifier) @identifier.attribute))

[
  "@required"
  "@lateinit"
] @identifier.attribute

[
  "="
  "??="
  ".="
  "|="
  "^="
  "&="
  "<<="
  ">>="
  "+="
  "-="
  "*="
  "/="
  "%="
  "**="
  "==>"
  "|>"
  "??"
  "||"
  "&&"
  "|"
  "^"
  "&"
  "=="
  "!="
  "==="
  "!=="
  "<"
  ">"
  "<="
  ">="
  "<=>"
  "<<"
  ">>"
  "->"
  "+"
  "-"
  "."
  "*"
  "/"
  "%"
  "**"
  "++"
  "--"
  "!"
  "?:"
  "="
  "??="
  ".="
  "|="
  "^="
  "&="
  "<<="
  ">>="
  "+="
  "-="
  "*="
  "/="
  "%="
  "**="
  "=>"
  ; type modifiers
  "@"
  "?"
  "~"
  "?->"
  "->"
] @operator

[
  (integer)
  (float)
] @number

(parameter
  (variable) @variable.parameter)

(call_expression
  function: (qualified_identifier
    (identifier) @keyword .)
    (#match? @keyword "(invariant|exit)"))

(call_expression
  function: (qualified_identifier
    (identifier) @function.builtin .)
    (#match? @function.builtin "(idx|is_int|is_bool|is_string)"))

(call_expression
  function: (qualified_identifier
    (identifier) @function.call .))

(call_expression
  function: (scoped_identifier
    (identifier) @function.call .))

(call_expression
  function: (selection_expression
    (qualified_identifier
      (identifier) @function.call .)))

 (xhp_class_identifier) @variable

(qualified_identifier
  (identifier) @identifier.module
  .
  (identifier))

; Handle built-ins not named in the tree sitter grammar
(qualified_identifier
  . (identifier) @constant.builtin  .
  (#match? @constant.builtin "(NAN|INF|__CLASS__|__DIR__|__FILE__|__FUNCTION__|__LINE__|__METHOD__|__NAMESPACE__|__TRAIT__)"))

; Explicitly handle internal amd module since they are not
; not mentioned in grammar
(qualified_identifier
  . (identifier) @keyword  .
  (#match? @keyword "(internal|module)"))

(namespace_declaration
    name:  (qualified_identifier (identifier) @identifier.module))

;; Mark import path components as namespaces by default
(use_statement
  (qualified_identifier
    (identifier) @identifier.module)
  (use_clause))

(use_statement
  (use_type
    "namespace")
  (use_clause
    (qualified_identifier
      (identifier) @identifier.module .)
    alias: (identifier)? @identifier.module))

(use_statement
  (use_type
    "const")
  (use_clause
    (qualified_identifier
      (identifier) @constant .)
    alias: (identifier)? @constant))

(use_statement
  (use_type
    "function")
  (use_clause
    (qualified_identifier
      (identifier) @function .)
    alias: (identifier)? @function))

(use_statement
  (use_type
    "type")
  (use_clause
    (qualified_identifier
      (identifier) @type .)
    alias: (identifier)? @type))

(use_clause
  (use_type
    "namespace")
  (qualified_identifier
    (identifier) @identifier.module .)
  alias: (identifier)? @identifier.module)

(use_clause
  (use_type
    "function")
  (qualified_identifier
    (identifier) @function .)
  alias: (identifier)? @function)

(use_clause
  (use_type
    "const")
  (qualified_identifier
    (identifier) @constant .)
  alias: (identifier)? @constant)

(use_clause
  (use_type
    "type")
  (qualified_identifier
    (identifier) @type .)
  alias: (identifier)? @type)

(function_declaration
  name: (identifier) @function)

(method_declaration
  name: (identifier) @function.method)


(type_arguments
  [
    "<"
    ">"
  ] @punctuation.bracket)

[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
  "<<"
  ">>"
] @punctuation.bracket

(ternary_expression
  [
    "?"
    ":"
  ] @identifier.operator)

[
  "."
  ";"
  "::"
  ":"
  ","
] @punctuation.delimiter

(qualified_identifier
  "\\" @punctuation.delimiter)

[
  (string)
  (xhp_string)
  (heredoc)
] @string

(xhp_open (xhp_identifier) @tag  (xhp_attribute)? @tag.attribute)
(xhp_close (xhp_identifier) @tag  (xhp_attribute)? @tag.attribute)
(xhp_open_close (xhp_identifier) @tag  (xhp_attribute)? @tag.attribute)

(type_specifier (qualified_identifier (identifier) @type))
(type_specifier) @type

(variable) @variable
(identifier) @variable
(pipe_variable) @variable
