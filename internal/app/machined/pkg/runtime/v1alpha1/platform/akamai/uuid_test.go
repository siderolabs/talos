// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package akamai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestGenerateLinodeUUID(t *testing.T) {
	tests := []struct {
		name     string
		linodeID int
		expected string
	}{
		{
			name:     "small ID",
			linodeID: 123,
			expected: "00000000-0000-0000-0000-000000000123",
		},
		{
			name:     "medium ID",
			linodeID: 79475478,
			expected: "00000000-0000-0000-0000-000079475478",
		},
		{
			name:     "large ID",
			linodeID: 999999999999,
			expected: "00000000-0000-0000-0000-999999999999",
		},
		{
			name:     "single digit",
			linodeID: 1,
			expected: "00000000-0000-0000-0000-000000000001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateLinodeUUID(tt.linodeID)
			assert.Equal(t, tt.expected, result)

			// Verify it's a valid UUID format (length and dashes)
			assert.Len(t, result, 36, "UUID should be 36 characters long")
			assert.Equal(t, byte('-'), result[8], "8th character should be dash")
			assert.Equal(t, byte('-'), result[13], "13th character should be dash")
			assert.Equal(t, byte('-'), result[18], "18th character should be dash")
			assert.Equal(t, byte('-'), result[23], "23rd character should be dash")
		})
	}
}

func TestIsInvalidUUID(t *testing.T) {
	tests := []struct {
		name     string
		uuid     string
		expected bool
	}{
		{
			name:     "empty string",
			uuid:     "",
			expected: true,
		},
		{
			name:     "all zeros",
			uuid:     "00000000-0000-0000-0000-000000000000",
			expected: true,
		},
		{
			name:     "valid UUID",
			uuid:     "550e8400-e29b-41d4-a716-446655440000",
			expected: false,
		},
		{
			name:     "generated Linode UUID",
			uuid:     "00000000-0000-0000-0000-000079475478",
			expected: false,
		},
		{
			name:     "another valid UUID",
			uuid:     "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInvalidUUID(tt.uuid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnsureValidUUID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("no override needed for valid UUID", func(t *testing.T) {
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		a := &Akamai{}

		// Create system info with valid UUID
		systemInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
		systemInfo.TypedSpec().UUID = "550e8400-e29b-41d4-a716-446655440000"
		require.NoError(t, st.Create(ctx, systemInfo))

		// Should not create UUID override
		err := a.ensureValidUUID(ctx, st, 79475478)
		require.NoError(t, err)

		// Verify no UUID override was created
		_, err = st.Get(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride)).Metadata())
		assert.True(t, state.IsNotFoundError(err), "UUID override should not exist for valid UUID")
	})

	t.Run("override created for zero UUID", func(t *testing.T) {
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		a := &Akamai{}

		// Create system info with zero UUID
		systemInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
		systemInfo.TypedSpec().UUID = "00000000-0000-0000-0000-000000000000"
		require.NoError(t, st.Create(ctx, systemInfo))

		// Should create UUID override
		err := a.ensureValidUUID(ctx, st, 79475478)
		require.NoError(t, err)

		// Verify UUID override was created with correct value
		metaKey, err := st.Get(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride)).Metadata())
		require.NoError(t, err)

		uuidKey := metaKey.(*runtimeres.MetaKey)
		assert.Equal(t, "00000000-0000-0000-0000-000079475478", uuidKey.TypedSpec().Value)
	})

	t.Run("override created when system info not available", func(t *testing.T) {
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		a := &Akamai{}

		// Don't create system info (simulates early boot)
		err := a.ensureValidUUID(ctx, st, 12345)
		require.NoError(t, err)

		// Verify UUID override was created
		metaKey, err := st.Get(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride)).Metadata())
		require.NoError(t, err)

		uuidKey := metaKey.(*runtimeres.MetaKey)
		assert.Equal(t, "00000000-0000-0000-0000-000000012345", uuidKey.TypedSpec().Value)
	})

	t.Run("existing override not modified", func(t *testing.T) {
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		a := &Akamai{}

		// Create existing UUID override
		existingUUID := "existing-uuid-should-not-change"
		uuidKey := runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride))
		uuidKey.TypedSpec().Value = existingUUID
		require.NoError(t, st.Create(ctx, uuidKey))

		// Create system info with zero UUID
		systemInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
		systemInfo.TypedSpec().UUID = "00000000-0000-0000-0000-000000000000"
		require.NoError(t, st.Create(ctx, systemInfo))

		// Should not modify existing override
		err := a.ensureValidUUID(ctx, st, 79475478)
		require.NoError(t, err)

		// Verify existing UUID override was not changed
		metaKey, err := st.Get(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride)).Metadata())
		require.NoError(t, err)

		uuidKeyResult := metaKey.(*runtimeres.MetaKey)
		assert.Equal(t, existingUUID, uuidKeyResult.TypedSpec().Value)
	})

	t.Run("no override for zero linodeID", func(t *testing.T) {
		st := state.WrapCore(namespaced.NewState(inmem.Build))
		a := &Akamai{}

		// Create system info with zero UUID
		systemInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
		systemInfo.TypedSpec().UUID = "00000000-0000-0000-0000-000000000000"
		require.NoError(t, st.Create(ctx, systemInfo))

		// Should not create override for invalid linodeID
		err := a.ensureValidUUID(ctx, st, 0)
		require.NoError(t, err)

		// Verify no UUID override was created
		_, err = st.Get(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride)).Metadata())
		assert.True(t, state.IsNotFoundError(err), "UUID override should not exist for zero linodeID")
	})
}

func TestEnsureValidUUIDIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	a := &Akamai{}

	// Test with invalid UUIDs that should trigger override
	invalidUUIDs := []string{
		"",
		"00000000-0000-0000-0000-000000000000",
	}

	for i, invalidUUID := range invalidUUIDs {
		t.Run(fmt.Sprintf("invalid_uuid_%d", i), func(t *testing.T) {
			// Create fresh state for each test
			testSt := state.WrapCore(namespaced.NewState(inmem.Build))

			if invalidUUID != "" {
				systemInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
				systemInfo.TypedSpec().UUID = invalidUUID
				require.NoError(t, testSt.Create(ctx, systemInfo))
			}

			linodeID := 1000 + i
			err := a.ensureValidUUID(ctx, testSt, linodeID)
			require.NoError(t, err)

			// Verify override was created
			metaKey, err := testSt.Get(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride)).Metadata())
			require.NoError(t, err)

			uuidKey := metaKey.(*runtimeres.MetaKey)
			expectedUUID := fmt.Sprintf("00000000-0000-0000-0000-%012d", linodeID)
			assert.Equal(t, expectedUUID, uuidKey.TypedSpec().Value)
		})
	}
}
