// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package modes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// ValidateStruct validates if the non-zero fields of a struct follow the mode
// requirement set for them.
func (c *Checker) ValidateStruct(s interface{}) error {
	return c.validateStruct("", s, nil)
}

// validateStruct recursevely checks a struct for their supported `modes` tags .
//   - `path is the history of fields formatted as `field1 -> field2 -> field2`.
//   - `element` is the current object being validated.
//   - `modesHistory` is a list of sets of modes seen so far in the ancestors of `element`.
func (c *Checker) validateStruct(path string, element interface{}, modesHistory []*set) error {
	v := reflect.ValueOf(element)
	t := reflect.TypeOf(element)

	// Check for nil pointers or other unset (zero) values.
	// Return early since it's OK if they are not set.
	if !v.IsValid() {
		return nil
	}

	switch v.Kind() {
	case reflect.Ptr:
		// If we're at a pointer, we need to dereference it.
		return c.validateStruct(path, v.Elem().Interface(), modesHistory)

	case reflect.Slice:
		// If we're at a slice, we recursevely validate each of its elements,
		// accumulating any error.
		var mErr *multierror.Error
		for i := 0; i < v.Len(); i++ {
			mErr = multierror.Append(mErr, c.validateStruct(path, v.Index(i).Interface(), modesHistory))
		}
		return mErr.ErrorOrNil()

	case reflect.Struct:
		// If we're at a struct, we interate over its fields, collecting their
		// HCL name to expand the path and any mode restriction to expand the
		// set of ancestors modes.
		var mErr *multierror.Error
		for i := 0; i < v.NumField(); i++ {
			// Ignore fields that are not set.
			fieldValue := v.Field(i)
			if fieldValue.IsZero() {
				continue
			}

			// Read the HCL tags in the field and ignore it if it doesn't have any.
			fieldType := t.Field(i)
			hclTags := fieldType.Tag.Get("hcl")
			if hclTags == "" {
				continue
			}

			// Extract HCL field name from tags and append to the current path.
			configName := strings.Split(hclTags, ",")[0]
			fullPath := configName
			if path != "" {
				fullPath = fmt.Sprintf("%s -> %s", path, configName)
			}

			// We need to take in considaration any mode set in this field and
			// in any of its ancestors.
			modesTag := fieldType.Tag.Get("modes")
			modesList := strings.Split(modesTag, ",")
			modeSet := newSet(modesList...)

			// Use a new slice so we don't modify the input.
			newHistory := []*set{modeSet}
			for _, s := range modesHistory {
				if s.len() > 0 {
					newHistory = append(newHistory, s)
				}
			}

			// Recurse on the field value.
			mErr = multierror.Append(mErr, c.validateStruct(fullPath, fieldValue.Interface(), newHistory))
		}
		return mErr.ErrorOrNil()

	default:
		// We reached a leaf, so check the mode history.
		// Our own mode was added to the history when we interated over the
		// struct fields.
		for _, s := range modesHistory {
			if !c.isAllowed(s.values()) {
				return fmt.Errorf("%s is only supported in %s", path, c.supportedModesMsg(s.values()))
			}
		}
		return nil
	}
}
