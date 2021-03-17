// +build !ent

package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
)

var (
	sep = " -> "

	// entOnlyConfig stores all the configuration values that are only allowed
	// in the Enterprise version of the Autoscaler.
	// To add a new value, use the `sep` value above between struct levels.
	// Slice types don't need to have index (e.g., "apm -> driver").
	entOnlyConfig = []string{
		"dynamic_application_sizing -> metrics_preload_threshold",
		"dynamic_application_sizing -> evaluate_after",
		"dynamic_application_sizing -> namespace_label",
		"dynamic_application_sizing -> job_label",
		"dynamic_application_sizing -> group_label",
		"dynamic_application_sizing -> task_label",
		"dynamic_application_sizing -> cpu_metric",
		"dynamic_application_sizing -> memory_metric",
	}
)

// DefaultEntConfig allows configuring enterprise only default configuration
// values.
func DefaultEntConfig() *Agent { return &Agent{} }

// ValidateEnt checks if Enterprise-only fields have been set.
func (a *Agent) ValidateEnt() error {
	return validateEntSubtree("", a)
}

// validateEntSubtree recursevely checks for Enterprise-only fields that have
// been set.
func validateEntSubtree(path string, subtree interface{}) error {
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
			fullPath = fmt.Sprintf("%s%s%s", path, sep, configName)
		}

		// If we are at an HCL block, we must recurse.
		if strings.Contains(hclTags, "block") {
			switch tf.Type.Kind() {
			case reflect.Slice:
				// Blocks can be defined multiple times, in which case they are
				// represented as a slice, so we recurse over all values in the
				// slice.
				for i := 0; i < vf.Len(); i++ {
					mErr = multierror.Append(mErr, validateEntSubtree(fullPath, vf.Index(i).Interface()))
				}

			default:
				// Otherwise just recurse with the value itself.
				mErr = multierror.Append(mErr, validateEntSubtree(fullPath, vf.Interface()))
			}
			continue
		}

		// We're finally at a config value, so check if it's marked as
		// Enterprise only.
		if isEntOnlyConfig(fullPath) {
			mErr = multierror.Append(mErr, fmt.Errorf("%s is only supported in Nomad Autoscaler Enterprise", fullPath))
		}
	}

	return mErr.ErrorOrNil()
}

func isEntOnlyConfig(path string) bool {
	for _, c := range entOnlyConfig {
		if path == c {
			return true
		}
	}

	return false
}
