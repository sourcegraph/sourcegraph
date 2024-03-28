; Types

(clazz (identifier) @type)
(typeAlias (identifier) @type)
((identifier) @type
 (#match? @type "^[A-Z]"))

(typeArgumentList
  "<" @punctuation.bracket
  ">" @punctuation.bracket)

; Method calls

(methodCallExpr
  (identifier) @function.method)

; Method definitions

(classMethod (methodHeader (identifier)) @function.method)
(objectMethod (methodHeader (identifier)) @function.method)

; Identifiers

(classProperty (identifier) @property)
(objectProperty (identifier) @property)

(parameterList (typedIdentifier (identifier) @variable.parameter))
(objectBodyParameters (typedIdentifier (identifier) @variable.parameter))

(identifier) @variable

; Literals

(stringConstant) @string
(slStringLiteral) @string
(mlStringLiteral) @string

(escapeSequence) @escape

(intLiteral) @number
(floatLiteral) @number

(interpolationExpr
  "\\(" @punctuation.special
  ")" @punctuation.special) @embedded

(interpolationExpr
 "\\#(" @punctuation.special
 ")" @punctuation.special) @embedded

(interpolationExpr
  "\\##(" @punctuation.special
  ")" @punctuation.special) @embedded

(lineComment) @comment
(blockComment) @comment
(docComment) @comment

; Operators

"??" @operator
"@"  @operator
"="  @operator
"<"  @operator
">"  @operator
"!"  @operator
"==" @operator
"!=" @operator
"<=" @operator
">=" @operator
"&&" @operator
"||" @operator
"+"  @operator
"-"  @operator
"**" @operator
"*"  @operator
"/"  @operator
"~/" @operator
"%"  @operator
"|>" @operator

"?"  @operator.type
"|"  @operator.type
"->" @operator.type

"," @punctuation.delimiter
":" @punctuation.delimiter
"." @punctuation.delimiter
"?." @punctuation.delimiter

"(" @punctuation.bracket
")" @punctuation.bracket
"[" @punctuation.bracket
"]" @punctuation.bracket
"{" @punctuation.bracket
"}" @punctuation.bracket

; Keywords

"abstract" @keyword
"amends" @keyword
"as" @keyword
"class" @keyword
"else" @keyword
"extends" @keyword
"external" @keyword
(falseLiteral) @constant.builtin
"for" @keyword
"function" @keyword
"hidden" @keyword
"if" @keyword
(importExpr "import" @function.method.builtin)
(importGlobExpr "import*" @function.method.builtin)
"import" @keyword
"import*" @keyword
"in" @keyword
"is" @keyword
"let" @keyword
"local" @keyword
(moduleExpr "module" @type.builtin)
"module" @keyword
"new" @keyword
"nothing" @type.builtin
(nullLiteral) @constant.builtin
"open" @keyword
"out" @keyword
(outerExpr) @variable.builtin
"read" @function.method.builtin
"read?" @function.method.builtin
"read*" @function.method.builtin
"super" @variable.builtin
(thisExpr) @variable.builtin
"throw" @function.method.builtin
"trace" @function.method.builtin
(trueLiteral) @constant.builtin
"typealias" @keyword
"unknown" @type.builtin
"when" @keyword
