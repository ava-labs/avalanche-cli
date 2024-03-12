// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import "testing"

func TestIsSSHKey(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "Valid RSA key",
			key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC0pBe3b2m5zJKvVlWfk7F0uRcxJ5LnA73LJ2+AW+JQLCRg5P5RPnRg3U4aV4n07a/x33UvCff3Dv+5G2E7QKQGxLizHcBKkE1dFpnO5BPNjSFK/4q+TFDdgA2YC47PODqDxXzOdb+et+1db/f4wYfPgqF2n1A1UXkG5pSzxNzMWvMEW6LeqA5Zq8cVnR51fESsWGDoqAptZ0J2B7s/UMGMbhZZqWflP1p6gAV3dpePC3F2Qf/SjCXHh4rpqDvHBLR4IKmI0zRiZ/Vq+H7Z39a6zXNyAT4PN/YCrX2q4VfljE4oJH3MC6+Vvjfg3tzRZkFshIJg0K4tOP1zqDAFj user@hostname",
			expected: true,
		},
		{
			name:     "Valid ed25519 key",
			key:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMiP5BOZuRjjE7V/HjDmGtBf/2YhUoZ1Fn5O8ss+nG3",
			expected: true,
		},
		{
			name:     "Valid ecdsa key",
			key:      "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBGe3ZjpyUBwM/43Alp2H7zQd48+4mV+//kjcsdqJcIK1FwR7d+8tjLjrUq8Ow2V1YLZMfdk9sC8Knyyl8Z5+Y=",
			expected: true,
		},
		{
			name:     "Invalid key",
			key:      "ssh-rsa-invalid AAAAB3NzaC1yc2EAAAADAQABAAABAQC0pBe3b2m5zJKvVlWfk7F0uRcxJ5LnA73LJ2+AW+JQLCRg5P5RPnRg3U4aV4n07a/x33UvCff3Dv+5G2E7QKQGxLizHcBKkE1dFpnO5BPNjSFK/4q+TFDdgA2YC47PODqDxXzOdb+et+1db/f4wYfPgqF2n1A1UXkG5pSzxNzMWvMEW6LeqA5Zq8cVnR51fESsWGDoqAptZ0J2B7s/UMGMbhZZqWflP1p6gAV3dpePC3F2Qf/SjCXHh4rpqDvHBLR4IKmI0zRiZ/Vq+H7Z39a6zXNyAT4PN/YCrX2q4VfljE4oJH3MC6+Vvjfg3tzRZkFshIJg0K4tOP1zqDAFj user@hostname",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsSSHPubKey(tc.key)
			if result != tc.expected {
				t.Errorf("Expected %v for key '%s', but got %v", tc.expected, tc.key, result)
			}
		})
	}
}
