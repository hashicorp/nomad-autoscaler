package blocking

// IndexHasChanged is used to check whether a returned blocking query has an
// updated index, compared to a tracked value.
func IndexHasChanged(new, old uint64) bool { return new > old }

// FindMaxFound is used to determine which value passed is the greatest. This
// is used to track the most recently found highest index value.
func FindMaxFound(new, old uint64) uint64 {
	if new <= old {
		return old
	}
	return new
}
