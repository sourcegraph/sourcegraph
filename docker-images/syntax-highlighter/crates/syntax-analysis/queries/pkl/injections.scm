; this definition is imprecise in that
; * any qualified or unqualified call to a method named "Regex" is considered a regex
; * string delimiters are considered part of the regex
(
  ((methodCallExpr (identifier) @methodName (argumentList (slStringLiteral) @injection.content))
    (#set! injection.language "regex"))
  (#eq? @methodName "Regex"))

; TODO: inject markdown into doc comments
