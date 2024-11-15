package util

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
)

// convert map to properties string
func ToProperties(conf map[string]string) string {
	// sort by key
	keys := maps.Keys(conf)
	slices.Sort(keys)

	// map to properties
	properties := make([]string, 0, len(conf)) // Pre-allocate properties slice
	for _, k := range keys {
		properties = append(properties, fmt.Sprintf("%s=%s", k, conf[k]))
	}
	return strings.Join(properties, "\n")
}
