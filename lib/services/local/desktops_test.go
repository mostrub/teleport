/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestListWindowsDesktops(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewWindowsDesktopService(bk)

	// Initially we expect no desktops.
	out, err := service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 10,
	})
	require.NoError(t, err)
	require.Empty(t, out.Desktops)
	require.Empty(t, out.NextKey)

	// Upsert some windows desktops.

	// With label.
	testLabel := map[string]string{"env": "test"}
	d1, err := types.NewWindowsDesktopV3("apple", testLabel, types.WindowsDesktopSpecV3{Addr: "_"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d1))

	d2, err := types.NewWindowsDesktopV3("banana", testLabel, types.WindowsDesktopSpecV3{Addr: "_"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d2))

	// Without labels.
	d3, err := types.NewWindowsDesktopV3("carrot", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d3))

	// Test fetch all.
	out, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 10,
	})
	require.NoError(t, err)
	require.Empty(t, out.NextKey)
	require.Empty(t, cmp.Diff([]types.WindowsDesktop{d1, d2, d3}, out.Desktops,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Test pagination.

	// First fetch.
	resp, err := service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 1,
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, out.Desktops[0], resp.Desktops[0])
	require.Equal(t, backend.GetPaginationKey(out.Desktops[1]), resp.NextKey)

	// Middle fetch.
	resp, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit:    1,
		StartKey: resp.NextKey,
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, out.Desktops[1], resp.Desktops[0])
	require.Equal(t, backend.GetPaginationKey(out.Desktops[2]), resp.NextKey)

	// Last fetch.
	resp, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit:    1,
		StartKey: resp.NextKey,
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, out.Desktops[2], resp.Desktops[0])
	require.Empty(t, resp.NextKey)
}

func TestListWindowsDesktops_Filters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewWindowsDesktopService(bk)

	// Upsert some windows desktops.

	// With label.
	testLabel := map[string]string{"env": "test"}
	d1, err := types.NewWindowsDesktopV3("banana", testLabel, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d1))

	d2, err := types.NewWindowsDesktopV3("banana", testLabel, types.WindowsDesktopSpecV3{Addr: "_"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d2))

	// Without labels.
	d3, err := types.NewWindowsDesktopV3("carrot", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d3))

	tests := []struct {
		name    string
		filter  types.ListWindowsDesktopsRequest
		wantErr bool
	}{
		{
			name: "NOK non-matching host id and name",
			filter: types.ListWindowsDesktopsRequest{
				Limit: 10,
				WindowsDesktopFilter: types.WindowsDesktopFilter{
					HostID: "no-match",
					Name:   "no-match",
				},
			},
			wantErr: true,
		},
		{
			name:    "NOK invalid limit",
			filter:  types.ListWindowsDesktopsRequest{},
			wantErr: true,
		},
		{
			name: "matching host id",
			filter: types.ListWindowsDesktopsRequest{
				Limit:                5,
				WindowsDesktopFilter: types.WindowsDesktopFilter{HostID: "test-host-id"},
			},
		},
		{
			name: "matching name",
			filter: types.ListWindowsDesktopsRequest{
				Limit:                5,
				WindowsDesktopFilter: types.WindowsDesktopFilter{Name: "banana"},
			},
		},
		{
			name: "with search",
			filter: types.ListWindowsDesktopsRequest{
				Limit:          5,
				SearchKeywords: []string{"env", "test"},
			},
		},
		{
			name: "with labels",
			filter: types.ListWindowsDesktopsRequest{
				Limit:  5,
				Labels: testLabel,
			},
		},
		{
			name: "with predicate",
			filter: types.ListWindowsDesktopsRequest{
				Limit:               5,
				PredicateExpression: `labels.env == "test"`,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := service.ListWindowsDesktops(ctx, tc.filter)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, resp.Desktops, 2)
			}
		})
	}
}
