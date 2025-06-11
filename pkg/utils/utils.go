package utils

func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[string]int)

	for _, val := range a {
		counts[val]++
	}

	for _, val := range b {
		counts[val]--
		if counts[val] < 0 {
			return false
		}
	}

	return true
}

func StringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valA := range a {
		if valB, ok := b[key]; !ok || valA != valB {
			return false
		}
	}

	return true
}
