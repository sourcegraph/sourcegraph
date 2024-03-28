package util

func Strp(v *string) string {
	if v == nil {
		return ""
	}

	return *v
}

func Intp(v *int) int {
	if v == nil {
		return 0
	}

	return *v
}
