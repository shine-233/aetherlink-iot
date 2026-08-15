package server

import "testing"

func TestPacketBytesCopyIncludesAuth(t *testing.T) {
	original := PacketBytes{Auth: 17, Total: 17}

	copied := original.copy()

	if copied.Auth != original.Auth {
		t.Fatalf("copy().Auth = %d, want %d", copied.Auth, original.Auth)
	}
	if copied.Total != original.Total {
		t.Fatalf("copy().Total = %d, want %d", copied.Total, original.Total)
	}
}
