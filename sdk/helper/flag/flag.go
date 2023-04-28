// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package flag

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// StringFlag implements the flag.Value interface and allows multiple calls to
// the same variable to append a list.
type StringFlag []string

func (s *StringFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *StringFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// FuncDurationVar is a type of flag that accepts a function, converts the
// user's value to a duration, and then calls the given function.
type FuncDurationVar func(d time.Duration) error

func (f FuncDurationVar) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	return f(v)
}
func (f FuncDurationVar) String() string   { return "" }
func (f FuncDurationVar) IsBoolFlag() bool { return false }

// FuncMapStringIngVar is a type of flag that accepts a function, converts the
// user's value to a map[string]int, and then calls the given function.
// User input should be in the <k1>:<v1>,<k2>:<v2>,... format.
type FuncMapStringIngVar func(m map[string]int) error

func (f FuncMapStringIngVar) Set(s string) error {
	m := make(map[string]int)

	for _, kv := range strings.Split(s, ",") {
		parts := strings.Split(kv, ":")
		if len(parts) != 2 {
			return fmt.Errorf("%q should be in <key>:<value> format", kv)
		}

		k := parts[0]
		v, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("%q is not a number", parts[1])
		}

		m[k] = v
	}
	return f(m)
}

func (f FuncMapStringIngVar) String() string { return "" }
