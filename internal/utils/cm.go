package utils

import "strings"

func ExtractWatchedKindsFromCM(cmData map[string]string) map[string][]string {
	watchedKinds := map[string][]string{}
	for k, v := range cmData {
		watchedKinds[k] = strings.Split(v, ",")
	}
	return watchedKinds
}

func HasRawKind(cmData map[string]string, k, v string) bool {
	val, containsNS := cmData[k]
	if !containsNS {
		return false
	}

	split := strings.Split(val, ",")
	for _, s := range split {
		if s == v {
			return true
		}
	}
	return false
}

func HasKind(watched map[string][]string, k, v string) bool {
	val, containNS := watched[k]

	if !containNS {
		return false
	}

	for _, s := range val {
		if s == v {
			return true
		}
	}
	return false
}
