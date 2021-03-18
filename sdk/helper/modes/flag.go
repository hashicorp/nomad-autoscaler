package modes

import (
	"flag"
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// Flag wraps the function call to set a flag to register it wit specific
// modes requirements.
func (c *Checker) Flag(name string, modes []string, fn func(string)) {
	c.flags[name] = modes
	fn(name)
}

// ValidateFlags validates that all flags that have been set fulfill the mode
// requirements.
func (c *Checker) ValidateFlags(flags *flag.FlagSet) error {
	var mErr *multierror.Error

	flags.Visit(func(flag *flag.Flag) {
		modes, ok := c.flags[flag.Name]
		if !ok || len(modes) == 0 {
			return
		}

		if !c.isAllowed(modes) {
			err := fmt.Errorf("-%s is only supported in %s", flag.Name, c.supportedModesMsg(modes))
			mErr = multierror.Append(mErr, err)
		}
	})

	return mErr.ErrorOrNil()
}
