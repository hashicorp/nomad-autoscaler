package blocking

// IndexHasChanged is used to check whether a returned blocking query has an
// updated index, compared to a tracked value.
func IndexHasChanged(new, old uint64) bool { return new != old }
