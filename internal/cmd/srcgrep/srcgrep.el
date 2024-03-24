(defun srcgrep (&optional query)
    "Runs srcgrep in the current directory with QUERY."
    (interactive "sQuery: ")
    (compile (concat "srcgrep --vimgrep " (shell-quote-argument query))))

; (global-set-key "\C-cs" 'srcgrep)
