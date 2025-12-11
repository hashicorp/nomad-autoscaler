// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package blocking

// IndexHasChanged is used to check whether a returned blocking query has an
// updated index, compared to a tracked value.
func IndexHasChanged(new, old uint64) bool { return new != old }
