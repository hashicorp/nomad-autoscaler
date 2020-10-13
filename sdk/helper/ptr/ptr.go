package ptr

func BoolToPtr(b bool) *bool {
	return &b
}

func IntToPtr(i int) *int {
	return &i
}

func Int64ToPtr(i int64) *int64 {
	return &i
}

func StringToPtr(s string) *string {
	return &s
}
