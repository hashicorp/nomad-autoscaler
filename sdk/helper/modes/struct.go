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

// validateStruct recursevely checks a struct for the supported `modes` tags .
func (c *Checker) validateStruct(path string, subtree interface{}, ancestorsModes *set) error {
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

	// We only support validating structs.
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

		// Extract HCL field name from tags and append to the current path.
		configName := strings.Split(hclTags, ",")[0]
		fullPath := configName
		if path != "" {
			fullPath = fmt.Sprintf("%s -> %s", path, configName)
		}

		// If we are at an HCL block, we must recurse and move to the next field.
		if strings.Contains(hclTags, "block") {
			mErr = multierror.Append(mErr, c.validateBlock(fullPath, tf, vf, ancestorsModes))
			continue
		}

		// We're finally at a config value, so check its required modes.
		mErr = multierror.Append(mErr, c.validateField(fullPath, tf, ancestorsModes))
	}

	return mErr.ErrorOrNil()
}

func (c *Checker) validateField(path string, field reflect.StructField, ancestorsModes *set) error {
	modesTag := field.Tag.Get("modes")

	// Ignore field if it or any of its ancestors don't have any mode restriction.
	if modesTag == "" && ancestorsModes.len() == 0 {
		return nil
	}

	// We need to take in considaration any mode set in this field and all in
	// any of its ancestors.
	modesList := strings.Split(modesTag, ",")
	modesSet := newSet(modesList...)
	modesSet.merge(ancestorsModes)

	if !c.isAllowed(modesSet.values()) {
		return fmt.Errorf("%s is only supported in %s", path, c.supportedModesMsg(modesSet.values()))
	}

	return nil
}

func (c *Checker) validateBlock(path string, blockField reflect.StructField, blockValue reflect.Value, ancestorsModes *set) error {
	// We need to take in considaration any mode set in this field and all in
	// any of its ancestors.
	modesTag := blockField.Tag.Get("modes")
	modesList := strings.Split(modesTag, ",")
	modesSet := newSet(modesList...)
	modesSet.merge(ancestorsModes)

	switch blockField.Type.Kind() {
	case reflect.Slice:
		// Blocks can be defined multiple times, in which case they are stored
		// as a slice, so we recurse over all values in the slice, accumulating
		// any error.
		var mErr *multierror.Error
		for i := 0; i < blockValue.Len(); i++ {
			mErr = multierror.Append(mErr, c.validateStruct(path, blockValue.Index(i).Interface(), modesSet))
		}
		return mErr.ErrorOrNil()

	default:
		// Otherwise just recurse with the block itself.
		return c.validateStruct(path, blockValue.Interface(), modesSet)
	}
}
