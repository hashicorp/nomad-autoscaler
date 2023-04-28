// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ptr

func BoolToPtr(b bool) *bool {
	return &b
}

func IntToPtr(i int) *int {
	return &i
}

func Int32ToPtr(i int32) *int32 {
	return &i
}

func Int64ToPtr(i int64) *int64 {
	return &i
}

func StringToPtr(s string) *string {
	return &s
}

func StringArrToPtr(s []string) *[]string {
	return &s
}

func PtrToInt64(i *int64) int64 {
	return *i
}
