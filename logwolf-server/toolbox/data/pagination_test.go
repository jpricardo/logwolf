package data

import (
	"errors"
	"testing"
)

func TestPaginationParams_Validate(t *testing.T) {
	cases := []struct {
		name string
		p    PaginationParams
		ok   bool
	}{
		{"first page, default size", PaginationParams{Page: 1, PageSize: DefaultPageSize}, true},
		{"largest page size", PaginationParams{Page: 1, PageSize: MaxPageSize}, true},
		{"deepest page", PaginationParams{Page: MaxPage, PageSize: MaxPageSize}, true},
		{"page size over the cap", PaginationParams{Page: 1, PageSize: MaxPageSize + 1}, false},
		{"page size zero", PaginationParams{Page: 1, PageSize: 0}, false},
		{"negative page size", PaginationParams{Page: 1, PageSize: -5}, false},
		{"page zero", PaginationParams{Page: 0, PageSize: 10}, false},
		{"page past the cap", PaginationParams{Page: MaxPage + 1, PageSize: 10}, false},
		// (page-1)*pageSize would overflow int64 and reach Mongo as a negative skip.
		{"page that overflows the skip", PaginationParams{Page: 1 << 62, PageSize: 100}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.Validate()
			if tc.ok && err != nil {
				t.Errorf("Validate(%+v) = %v, want nil", tc.p, err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidPagination) {
				t.Errorf("Validate(%+v) = %v, want ErrInvalidPagination", tc.p, err)
			}
		})
	}
}

// TestAllLogs_RefusesInvalidPaginationBeforeQuerying: the bounds hold for any
// caller of the data layer, not only the broker. Models has no client here, so
// reaching the query would panic.
func TestAllLogs_RefusesInvalidPaginationBeforeQuerying(t *testing.T) {
	var m Models
	if _, err := m.AllLogs([12]byte{1}, PaginationParams{Page: 1, PageSize: 1_000_000}); !errors.Is(err, ErrInvalidPagination) {
		t.Errorf("AllLogs with a million-log page = %v, want ErrInvalidPagination", err)
	}
}
