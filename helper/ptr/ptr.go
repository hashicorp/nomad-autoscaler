package ptr

func BoolToPtr(b bool) *bool {
	return &b
}

func Int64ToPtr(i int64) *int64 {
	return &i
}
