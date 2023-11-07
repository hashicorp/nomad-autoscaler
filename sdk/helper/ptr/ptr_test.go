// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ptr

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_Of(t *testing.T) {

	s := "hello"
	sPtr := Of(s)

	must.Eq(t, s, *sPtr)

	b := "bye"
	sPtr = &b
	must.NotEq(t, s, *sPtr)
}
