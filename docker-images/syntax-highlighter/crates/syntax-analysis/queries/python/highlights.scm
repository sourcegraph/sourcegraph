; Function calls

(decorator) @identifier.function

(call
  function: (attribute attribute: (identifier) @identifier.function))
(call
  function: (identifier) @identifier.function)

; Function definitions

(function_definition
  name: (identifier) @identifier.function)

(identifier) @variable
(attribute attribute: (identifier) @identifier.attribute)
(type (identifier) @identifier.type)

; Literals

[(true) (false)] @boolean

(none) @constant.builtin

[
  (integer)
  (float)]
@number

(comment) @comment
(string) @string
(escape_sequence) @string.escape

(interpolation
  "{" @string.escape
  "}" @string.escape)

[
  "-"
  "-="
  "!="
  "*"
  "**"
  "**="
  "*="
  "/"
  "//"
  "//="
  "/="
  "&"
  "%"
  "%="
  "^"
  "+"
  "->"
  "+="
  "<"
  "<<"
  "<="
  "<>"
  "="
  ":="
  "=="
  ">"
  ">="
  ">>"
  "|"
  "~"]
@identifier.operator

[
  "and"
  "as"
  "assert"
  "async"
  "await"
  "break"
  "case"
  "class"
  "continue"
  "def"
  "del"
  "elif"
  "else"
  "except"
  "exec"
  "finally"
  "for"
  "from"
  "global"
  "if"
  "import"
  "in"
  "is"
  "lambda"
  "match"
  "nonlocal"
  "not"
  "or"
  "pass"
  "print"
  "raise"
  "return"
  "try"
  "while"
  "with"
  "yield"]
@keyword
