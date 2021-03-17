package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// mode represents a bitmaks of config `modes` struct tags.
type mode uint8

const (
	// This is the list of possible modes.
	// To add a new mode:
	//   * add a new entry below existing ones (you don't need to provide a type nor value)
	//   * add a new human-friendly name for the version mode in `modeNames`
	//   * map the `modes` struct tag value with the const in the `switch` statement of `fieldTagsToBitmask`
	MODE_NONE mode = 0
	MODE_ENT  mode = 1 << iota
)

var modeNames = []string{
	"",
	"Nomad Autoscaler Enterprise",
}

func fieldTagsToBitmask(tags string) mode {
	var result mode
	for _, t := range strings.Split(tags, ",") {
		switch t {
		case "ent":
			result |= MODE_ENT
		}
	}

	return result
}

// validateStructModes recursevely checks a config struct for the supported
// `modes` tags .
func validateStructModes(path string, subtree interface{}, ancestorsModes mode) error {
	var mErr *multierror.Error

	v := reflect.ValueOf(subtree)
	t := reflect.TypeOf(subtree)

	// Check for nil pointers or other unset (zero) values.
	// Return early since it's OK if they are not set.
	if !v.IsValid() {
		return nil
	}

	// If we're at a pointer, we need to dereference it.
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	// Iterate over the subfields.
	for i := 0; i < v.NumField(); i++ {

		// Ignore fields that are not set.
		vf := v.Field(i)
		if vf.IsZero() {
			continue
		}

		// Read the HCL tags in the field and ignore it if it doesn't have any.
		tf := t.Field(i)
		hclTags := tf.Tag.Get("hcl")
		if hclTags == "" {
			continue
		}

		// Extract HCL field name from tags and build the current full path.
		configName := strings.Split(hclTags, ",")[0]
		fullPath := configName
		if path != "" {
			fullPath = fmt.Sprintf("%s -> %s", path, configName)
		}

		// If we are at an HCL block, we must recurse.
		if strings.Contains(hclTags, "block") {

			// We need to keep all modes set in ancestors blocks plus our own mode restrictions.
			modes := ancestorsModes | fieldTagsToBitmask(tf.Tag.Get("modes"))

			switch tf.Type.Kind() {
			case reflect.Slice:
				// Blocks can be defined multiple times, in which case they are
				// represented as a slice, so we recurse over all values in the
				// slice.
				for i := 0; i < vf.Len(); i++ {
					mErr = multierror.Append(mErr, validateStructModes(fullPath, vf.Index(i).Interface(), modes))
				}

			default:
				// Otherwise just recurse with the value itself.
				mErr = multierror.Append(mErr, validateStructModes(fullPath, vf.Interface(), modes))
			}
			continue
		}

		// We're finally at a config value, so check its required modes.
		mErr = multierror.Append(mErr, validateField(fullPath, tf, ancestorsModes))
	}

	return mErr.ErrorOrNil()
}

func validateField(path string, field reflect.StructField, ancestorsModes mode) error {
	modes := field.Tag.Get("modes")

	// Ignore field if it or any of its ancestors have any mode restriction.
	if modes == "" && ancestorsModes == MODE_NONE {
		return nil
	}

	// We need to take in considaration any mode set in an ancestors block.
	modesMask := fieldTagsToBitmask(modes) | ancestorsModes
	result := modesMask & validModes

	// If there are not bits set after applying the mask it means that none of
	// the valid modes are present in the bitmask.
	if result == 0 {
		return fmt.Errorf("%s is only supported in %s", path, supportedVersionsMsg(modesMask))
	}

	return nil
}

// supportedVersionsMsg takes a bitmask of modes and returns a human-friendly
// string of supported version names.
func supportedVersionsMsg(modes mode) string {
	names := []string{}

	var m mode
	for i := 0; i < len(modeNames); i++ {
		m = 1 << i
		if m&modes != 0 {
			names = append(names, modeNames[i])
		}
	}

	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		init := names[0 : len(names)-1]
		last := names[len(names)-1]
		return fmt.Sprintf("%s, and %s", strings.Join(init, ", "), last)
	}
}
