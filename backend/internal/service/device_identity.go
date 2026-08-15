package service

import (
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
)

// normalizeOptionalRDIDeviceNumber maps historical RDI PID input to the current device number field.
func normalizeOptionalRDIDeviceNumber(deviceNumber **string, pidNumber *string) error {
	if pidNumber == nil || *pidNumber == "" {
		return nil
	}
	normalizedPIDNumber, err := NormalizeRDIPID(*pidNumber)
	if err != nil {
		return err
	}
	*deviceNumber = &normalizedPIDNumber
	return nil
}

// isValidDeviceID accepts only device identifiers that match the length and character constraints.
func isValidDeviceID(id string) bool {
	if len(id) < 8 || len(id) > 36 {
		return false
	}

	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}

	return true
}

func NormalizeRDIPID(pid string) (string, error) {
	pid = strings.ToUpper(strings.TrimSpace(pid))
	if pid == "" {
		return "", nil
	}
	if len(pid) != 12 {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "pid_number must be exactly 12 alphanumeric characters")
	}
	for _, r := range pid {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') {
			return "", errcode.NewWithMessage(errcode.CodeParamError, "pid_number must contain only letters and numbers")
		}
	}
	return pid, nil
}
