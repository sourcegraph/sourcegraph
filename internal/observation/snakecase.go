package observation

import (
	"strings"
)

// modified version from https://gist.github.com/hxsf/7f5392c0153d3a8607c42eefed02b8cd.
// Assumes ASCII to become a leaner version of the original that handled Unicode.
func toSnakeCase(s string) string {
	if len(s) == 0 {
		return ""
	}
	dist := strings.Builder{}
	dist.Grow(len(s) + len(s)/3) // avoid reallocation memory
	for i := range len(s) {
		cur := s[i]
		if cur == ' ' {
			continue
		}
		// if - or _: write _
		if cur == '-' || cur == '_' {
			dist.WriteByte('_')
			continue
		}

		// if lowercase, . or number: passthrough
		if (cur >= 'a' && cur <= 'z') || cur == '.' || ('0' <= cur && cur <= '9') {
			dist.WriteByte(cur)
			continue
		}

		// else if neither -, _, ., lowercase or a number, assume uppercase and lowercase it
		if i == 0 {
			dist.WriteByte(cur + 32)
			continue
		}

		last := s[i-1]

		// if not at the last one (at this stage, cur is assumed uppercase)
		if i < len(s)-1 {
			next := s[i+1]
			if next >= 'a' && next <= 'z' {
				isLastCapital := last >= 'A' && last <= 'Z'
				// specialize pluralized acronyms but not 'Is', so
				if cur == 'I' && next == 's' {
					dist.WriteByte('_')
				} else if last != '.' && last != '_' && last != '-' && (!isLastCapital || next != 's') {
					dist.WriteByte('_')
				}
				dist.WriteByte(cur + 32)
				continue
			}
		}
		if last >= 'a' && last <= 'z' {
			dist.WriteByte('_')
		}
		// last char is uppercase, lowercase it
		dist.WriteByte(cur + 32)
	}

	return dist.String()
}
