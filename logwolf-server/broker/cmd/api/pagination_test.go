package main

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"logwolf-toolbox/data"
)

func TestPaginationFromQuery(t *testing.T) {
	cases := []struct {
		query string
		want  data.PaginationParams
		ok    bool
	}{
		{"", data.PaginationParams{Page: 1, PageSize: data.DefaultPageSize}, true},
		{"page=3", data.PaginationParams{Page: 3, PageSize: data.DefaultPageSize}, true},
		{"page=2&pageSize=50", data.PaginationParams{Page: 2, PageSize: 50}, true},
		{"pageSize=100", data.PaginationParams{Page: 1, PageSize: 100}, true},
		// Given but unusable: refused, where it used to fall back to 20 quietly.
		{"pageSize=101", data.PaginationParams{}, false},
		{"pageSize=1000000", data.PaginationParams{}, false},
		{"pageSize=0", data.PaginationParams{}, false},
		{"pageSize=-1", data.PaginationParams{}, false},
		{"pageSize=abc", data.PaginationParams{}, false},
		{"pageSize=1.5", data.PaginationParams{}, false},
		{"page=0", data.PaginationParams{}, false},
		{"page=4611686018427387904", data.PaginationParams{}, false},
		{"page=99999999999999999999", data.PaginationParams{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			qp, _ := url.ParseQuery(tc.query)
			got, err := paginationFromQuery(qp)
			if !tc.ok {
				if !errors.Is(err, data.ErrInvalidPagination) {
					t.Errorf("paginationFromQuery(%q) = %+v, %v; want ErrInvalidPagination", tc.query, got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("paginationFromQuery(%q) = %+v, %v; want %+v", tc.query, got, err, tc.want)
			}
		})
	}
}

// TestLogReads_RefuseAnOversizedPage covers both read routes, the SDK's and the
// dashboard's: a page over the cap is a 400, and nothing reaches the logger.
func TestLogReads_RefuseAnOversizedPage(t *testing.T) {
	h, f := newInternalTestServer(t)
	key := seedKey(t, projAlpha, data.ScopeRead)

	sdk := do(h, keyRequest(http.MethodGet, "/logs?pageSize=1000", key, ""))
	if sdk.Code != http.StatusBadRequest {
		t.Errorf("GET /logs?pageSize=1000 = %d, want 400 (body: %s)", sdk.Code, sdk.Body.String())
	}

	dash := do(h, internalRequest(http.MethodGet, "/projects/"+projAlpha+"/logs?pageSize=1000", "member-a", nil))
	if dash.Code != http.StatusBadRequest {
		t.Errorf("GET /projects/{id}/logs?pageSize=1000 = %d, want 400 (body: %s)", dash.Code, dash.Body.String())
	}

	f.snapshot(func(f *fakeLogger) {
		if len(f.getLogsParams) != 0 {
			t.Errorf("GetLogs forwarded for an oversized page: %+v", f.getLogsParams)
		}
	})

	ok := do(h, internalRequest(http.MethodGet, "/projects/"+projAlpha+"/logs?page=2&pageSize=100", "member-a", nil))
	if ok.Code != http.StatusOK {
		t.Fatalf("GET /projects/{id}/logs?page=2&pageSize=100 = %d, want 200 (body: %s)", ok.Code, ok.Body.String())
	}
	f.snapshot(func(f *fakeLogger) {
		want := data.PaginationParams{Page: 2, PageSize: 100}
		if len(f.getLogsParams) != 1 || f.getLogsParams[0].Pagination != want {
			t.Errorf("GetLogs calls = %+v, want one with %+v", f.getLogsParams, want)
		}
	})
}
