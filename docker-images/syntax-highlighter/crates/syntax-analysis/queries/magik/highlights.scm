; Based on https://github.com/krn-robin/tree-sitter-magik/blob/main/queries/highlights.scm

; Methods
(method
  exemplarname: (identifier) @type)
(method
  name: (identifier) @function)

(method "." @operator)

(procedure (label) @function)

(documentation) @comment
(comment) @comment

"_package" @include
(package (identifier) @identifier.module)

; Expression
[
    ">>"
    ">"
    ">="
    "<"
    "<="
    "="
    "~="
    "<>"
    "+"
    "-"
    "/"
    "*"
    "**"
    "<<"
    "^<<"
    "_and<<"
    "_andif<<"
    "_or<<"
    "_orif<<"
    "_xor<<"
    "_xorif"
    "**<<"
    "**^<<"
    "*<<"
    "*^<<"
    "/<<"
    "/^<<"
    "_mod<<"
    "_div<<"
    "-<<"
    "-^<<"
    "+<<"
    "+^<<"
] @operator

(relational_operator
    operator: _ @operator)

(logical_operator
    operator: _ @operator)

(arithmetic_operator
    operator: _ @operator)

(unary_operator
    operator: _ @operator)

(invoke
  receiver: (variable) @function.builtin
  (#eq? @function.builtin "def_slotted_exemplar"))

(invoke
  receiver: (variable) @function.builtin
  (#eq? @function.builtin "def_mixin"))

(invoke
  receiver: (_) @function)
(call
  operator: "." @operator)
(call
  message: (identifier) @function
  "(")

; Keywords
[
  "_abstract"
  "_allresults"
  "_block"
  "_catch"
  "_cf"
  "_class"
  "_constant"
  "_continue"
  "_default"
  "_dynamic"
  "_elif"
  "_else"
  "_endblock"
  "_endcatch"
  "_endif"
  "_endlock"
  "_endloop"
  "_endmethod"
  "_endproc"
  "_endprotect"
  "_endtry"
  "_finally"
  "_for"
  "_gather"
  "_global"
  "_handling"
  "_if"
  "_import"
  "_iter"
  "_leave"
  "_local"
  "_lock"
  "_loop"
  "_loopbody"
  "_method"
  "_optional"
  "_over"
  "_primitive"
  "_private"
  "_proc"
  "_protect"
  "_protection"
  "_return"
  "_scatter"
  "_then"
  "_thisthread"
  "_throw"
  "_try"
  "_when"
  "_while"
  "_with"
] @keyword


(regex_literal) @string.special

; NOTE(issues: https://github.com/krn-robin/tree-sitter-magik/issues/49): Grammar does not support traversing to the children in the pragma
(pragma) @identifier.attribute

(argument) @identifier.parameter

; Literals
(number) @number

[
  (string_literal)
  (symbol)
] @string

(character_literal) @character

[(true) (false)] @boolean

(maybe) @constant.builtin

(unset) @constant.null

[
 (self)
 (super)
 (clone)
] @variable.builtin

[
 (variable)
 (dynamic_variable)
 (global_variable)
 (global_reference)
 (identifier)
 (slot_accessor)
 (label)
] @variable
