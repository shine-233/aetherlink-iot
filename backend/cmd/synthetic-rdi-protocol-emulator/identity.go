package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const (
	SyntheticMode       = "synthetic-rdi"
	ProtocolEmulatorTag = "protocol-emulator"
	ObservedProtocol    = "aetherlink-generic-device-contract-v1"
	SyntheticPIDPattern = `^SYN[A-Z0-9]{9}$`
)

// SyntheticVoucher and SyntheticHardwareIdentity deliberately describe test
// credentials and test hardware only.  They must never be presented as an
// RDI-issued voucher or physical identity.
type SyntheticVoucher struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SyntheticHardwareIdentity struct {
	Kind   string `json:"kind"`
	Serial string `json:"serial"`
}

type SyntheticIdentity struct {
	Mode       string                    `json:"mode"`
	Provenance string                    `json:"provenance"`
	PID        string                    `json:"pid"`
	DeviceID   string                    `json:"device_id"`
	Voucher    SyntheticVoucher          `json:"voucher"`
	Hardware   SyntheticHardwareIdentity `json:"hardware"`
}

type PublicManifest struct {
	EvidenceClass         string `json:"evidence_class"`
	FixtureProvenance     string `json:"fixture_provenance"`
	Protocol              string `json:"protocol"`
	PID                   string `json:"pid"`
	DeviceID              string `json:"device_id"`
	HardwareKind          string `json:"hardware_kind"`
	HardwareSerial        string `json:"hardware_serial"`
	VoucherUsername       string `json:"voucher_username"`
	VoucherSecretRedacted bool   `json:"voucher_secret_redacted"`
	DeviceExecution       string `json:"device_execution"`
	RealRDIStatus         string `json:"real_rdi_status"`
}

// PublicManifest is safe to print into a test artifact.  The voucher
// password remains available to an explicitly configured live process, but
// never becomes part of the evidence file or stdout manifest.
func (identity SyntheticIdentity) PublicManifest() PublicManifest {
	return PublicManifest{
		EvidenceClass:         ProtocolEmulatorTag,
		FixtureProvenance:     identity.Provenance,
		Protocol:              ObservedProtocol,
		PID:                   identity.PID,
		DeviceID:              identity.DeviceID,
		HardwareKind:          identity.Hardware.Kind,
		HardwareSerial:        identity.Hardware.Serial,
		VoucherUsername:       identity.Voucher.Username,
		VoucherSecretRedacted: true,
		DeviceExecution:       "not-proven",
		RealRDIStatus:         "not-tested",
	}
}

// GenerateSyntheticIdentity creates a stable, isolated identity for software
// contract tests.  The PID has the same 12-character alphanumeric shape that
// the repository's RDI normalizer accepts, but the provenance makes it
// impossible to mistake this value for a real activated controller.
func GenerateSyntheticIdentity(seed string) (SyntheticIdentity, error) {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return SyntheticIdentity{}, fmt.Errorf("synthetic identity seed is required")
	}

	digest := sha256.Sum256([]byte("aetherlink-synthetic-rdi:" + seed))
	hexDigest := hex.EncodeToString(digest[:])
	identity := SyntheticIdentity{
		Mode:       SyntheticMode,
		Provenance: SyntheticMode,
		PID:        "SYN" + strings.ToUpper(hexDigest[:9]),
		DeviceID:   "synthetic-device-" + hexDigest[:12],
		Voucher: SyntheticVoucher{
			Username: "synthetic-rdi-" + hexDigest[:16],
			Password: "synthetic-voucher-" + hexDigest[16:40],
		},
		Hardware: SyntheticHardwareIdentity{
			Kind:   "synthetic",
			Serial: "SYNTH-HW-" + strings.ToUpper(hexDigest[40:56]),
		},
	}
	return identity, nil
}

// OverrideSyntheticIdentity binds the protocol emulator to an explicitly
// seeded database fixture. The override is intentionally limited to the
// synthetic namespace so a caller cannot use this tool to manufacture a
// claim of a real RDI identity.
func OverrideSyntheticIdentity(identity SyntheticIdentity, pid, deviceID string) (SyntheticIdentity, error) {
	if err := validateSyntheticIdentity(identity); err != nil {
		return SyntheticIdentity{}, err
	}
	pid = strings.TrimSpace(pid)
	deviceID = strings.TrimSpace(deviceID)
	if pid == "" && deviceID == "" {
		return identity, nil
	}
	if pid != "" {
		if !regexp.MustCompile(SyntheticPIDPattern).MatchString(strings.ToUpper(pid)) {
			return SyntheticIdentity{}, fmt.Errorf("synthetic PID override must use the SYN namespace and contain exactly 12 alphanumeric characters, got %q", pid)
		}
		identity.PID = strings.ToUpper(pid)
	}
	if deviceID != "" {
		identity.DeviceID = deviceID
	}
	return identity, validateSyntheticIdentity(identity)
}

func validateSyntheticIdentity(identity SyntheticIdentity) error {
	if identity.Mode != SyntheticMode || identity.Provenance != SyntheticMode {
		return fmt.Errorf("only %q identity provenance is accepted, got mode=%q provenance=%q", SyntheticMode, identity.Mode, identity.Provenance)
	}
	if !regexp.MustCompile(SyntheticPIDPattern).MatchString(strings.ToUpper(identity.PID)) {
		return fmt.Errorf("synthetic PID must use the SYN namespace and contain exactly 12 alphanumeric characters, got %q", identity.PID)
	}
	if identity.DeviceID == "" || identity.Voucher.Username == "" || identity.Voucher.Password == "" {
		return fmt.Errorf("synthetic identity is incomplete")
	}
	if identity.Hardware.Kind != "synthetic" || identity.Hardware.Serial == "" {
		return fmt.Errorf("hardware identity must be explicitly synthetic")
	}
	return nil
}
