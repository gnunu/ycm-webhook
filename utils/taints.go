package utils

import v1 "k8s.io/api/core/v1"

// DeleteTaintsByKey removes all the taints that have the same key to given taintKey
func DeleteTaintsByKey(taints []v1.Taint, taintKey string) ([]v1.Taint, bool) {
	newTaints := []v1.Taint{}
	deleted := false
	for i := range taints {
		if taintKey == taints[i].Key {
			deleted = true
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, deleted
}

// TaintKeyExists checks if the given taint key exists in list of taints. Returns true if exists false otherwise.
func TaintKeyExists(taints []v1.Taint, taintKeyToMatch string) bool {
	for _, taint := range taints {
		if taint.Key == taintKeyToMatch {
			return true
		}
	}
	return false
}
