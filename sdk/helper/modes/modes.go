package modes

import (
	"fmt"
	"strings"
)

// Checker verifies if a set of CLI flags or the fields of an HCL struct
// object follow the required mode rules.
type Checker struct {
	modes   map[string]string
	enabled *set
	flags   map[string][]string
}

// NewChecker returns a new Checker with the given modes, some of them enabled.
func NewChecker(modes map[string]string, enabled []string) *Checker {
	return &Checker{
		modes:   modes,
		enabled: newSet(enabled...),
		flags:   make(map[string][]string),
	}
}

// isAllowed returns true if any of the modes provided are allowed.
func (c *Checker) isAllowed(modes []string) bool {
	if len(modes) == 0 {
		return true
	}

	for _, m := range modes {
		if c.enabled.contains(m) {
			return true
		}
	}

	return false
}

// supportedModesMsg takes a list of modes and returns a human-friendly string
// of supported mode names.
func (c *Checker) supportedModesMsg(modes []string) string {
	names := []string{}
	modesSet := newSet(modes...)

	for tag, name := range c.modes {
		if modesSet.contains(tag) {
			names = append(names, name)
		}
	}

	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return fmt.Sprintf("%s and %s", names[0], names[1])
	default:
		init := names[0 : len(names)-1]
		last := names[len(names)-1]
		return fmt.Sprintf("%s, and %s", strings.Join(init, ", "), last)
	}
}
