(require 'consult)

(defun consult-src (&optional dir initial)
  (interactive "P")
  (consult--grep "Src" #'consult--src-make-builder dir initial))

(defun consult--src-make-builder (paths)
  "Create grep command line builder given PATHS."
    (lambda (input)
        (let* ((cmd (consult--build-args "python3 /Users/keegan/src/github.com/sourcegraph/sourcegraph/srcgrep.py"))
                  (input-query (concat "repo:^github\\.com/sourcegraph/sourcegraph$ " input))
                  (terms (split-string input-query))
                  (query (string-join terms " AND ")))
            (cons (append cmd (list query))
                (apply-partially #'consult--highlight-regexps
                    (list (regexp-quote input)) t)))))
