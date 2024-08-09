(literal_suffix) @identifier
(identifier) @identifier
(namespace_identifier) @identifier.module
(field_identifier) @identifier.attribute
(statement_identifier) @identifier.attribute
(type_identifier) @type
(primitive_type) @type.builtin
(sized_type_specifier) @type.builtin
(static_assert_declaration ("static_assert") @identifier.builtin)
(attribute name: (identifier) @identifier.attribute)

(this) @constant.builtin
(comment) @comment
(operator_name "operator" @keyword)
(operator_name) @identifier
(auto) @keyword


(string_literal) @string
(system_lib_string) @string
(raw_string_literal) @string

(null) @constant.null
("nullptr") @constant.null
(number_literal) @number
(char_literal) @character
(true) @boolean
(false) @boolean

(call_expression
  function: (identifier) @identifier.function)
(call_expression
  function: (field_expression
             field: (field_identifier) @identifier.function))
(function_declarator
  declarator: [
    (identifier)
    (field_identifier)
  ] @identifier.function)

(destructor_name (identifier) @skip) @identifier.function
(preproc_function_def
  name: (identifier) @identifier.function)

[
  "#define"
  "#elif"
  "#else"
  "#endif"
  "#if"
  "#ifdef"
  "#ifndef"
  "#include"
  "break"
  "case"
  "const"
  "continue"
  "co_await"
  "co_return"
  "co_yield"
  "default"
  "delete"
  "class"
  "public"
  "protected"
  "private"
  "final"
  "friend"
  "goto"
  "do"
  "else"
  "enum"
  "explicit"
  "extern"
  "for"
  "if"
  "try"
  "catch"
  "throw"
  "inline"
  "namespace"
  "new"
  "noexcept"
  "return"
  "sizeof"
  "static"
  "struct"
  "decltype"
  "switch"
  "template"
  "typedef"
  "typename"
  "union"
  "using"
  "volatile"
  "constexpr"
  "while"
  (virtual)
  (preproc_directive)] @keyword
