package util

func ContainsString(elems []string, elem string) bool {
	for _, e := range elems {
		if e == elem {
			return true
		}
	}
	return false
}
