package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type otaProgressUpdate struct {
	progress    int16
	hasProgress bool
	status      int16
	hasStatus   bool
	description string
	version     string
}

func resolveOTAProgressUpdate(params map[string]interface{}) (otaProgressUpdate, bool) {
	progress, hasProgress := rdiOTAProgressFromParams(params)
	status, hasStatus := rdiOTAStatusFromParams(params, progress, hasProgress)
	if !hasStatus && !hasProgress {
		return otaProgressUpdate{}, false
	}

	return otaProgressUpdate{
		progress:    progress,
		hasProgress: hasProgress,
		status:      status,
		hasStatus:   hasStatus,
		description: rdiOTAStatusDescription(params, status, hasStatus, progress, hasProgress),
		version:     rdiOTAVersionFromParams(params),
	}, true
}

func otaProgressReachedCompletion(progressUpdate otaProgressUpdate) bool {
	return (progressUpdate.hasStatus && progressUpdate.status == 4) || (progressUpdate.hasProgress && progressUpdate.progress >= 100)
}

func rdiOTAProgressFromParams(params map[string]interface{}) (int16, bool) {
	for _, key := range []string{"progress", "step", "steps", "percent", "percentage"} {
		if value, ok := params[key]; ok {
			number, ok := rdiOTANumber(value)
			if !ok {
				continue
			}
			number = math.Max(0, math.Min(100, number))
			return int16(math.Round(number)), true
		}
	}
	return 0, false
}

func rdiOTAStatusFromParams(params map[string]interface{}, progress int16, hasProgress bool) (int16, bool) {
	for _, key := range []string{"status", "state", "result", "code"} {
		if value, ok := params[key]; ok {
			if status, ok := rdiOTAStatusValue(value, progress, hasProgress); ok {
				return status, true
			}
		}
	}
	if hasProgress {
		if progress >= 100 {
			return 4, true
		}
		if progress > 0 {
			return 3, true
		}
	}
	return 0, false
}

func rdiOTAStatusValue(value interface{}, progress int16, hasProgress bool) (int16, bool) {
	if status, ok := rdiOTAStatusFromNumber(value, progress, hasProgress); ok {
		return status, true
	}

	text := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	text = strings.ReplaceAll(text, "-", "_")
	text = strings.ReplaceAll(text, " ", "_")
	return rdiOTAStatusFromText(text)
}

func rdiOTAStatusFromNumber(value interface{}, progress int16, hasProgress bool) (int16, bool) {
	number, ok := rdiOTANumber(value)
	if !ok {
		return 0, false
	}
	status := int16(math.Round(number))
	if status >= 1 && status <= 6 {
		return status, true
	}
	if status == 0 && hasProgress {
		return rdiOTAStatusFromProgress(progress)
	}
	return 0, false
}

func rdiOTAStatusFromProgress(progress int16) (int16, bool) {
	if progress >= 100 {
		return 4, true
	}
	return 3, true
}

func rdiOTAStatusFromText(text string) (int16, bool) {
	switch text {
	case "queued", "pending", "waiting", "wait":
		return 1, true
	case "pushed", "notified", "issued", "sent":
		return 2, true
	case "upgrading", "upgrade", "in_progress", "inprogress", "progress", "downloading", "downloaded", "installing", "running":
		return 3, true
	case "success", "succeeded", "done", "completed", "complete", "finished", "finish", "ok":
		return 4, true
	case "fail", "failed", "error", "timeout", "aborted":
		return 5, true
	case "cancel", "canceled", "cancelled":
		return 6, true
	default:
		return 0, false
	}
}

func rdiOTANumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		number, err := v.Float64()
		return number, err == nil
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func rdiOTAStatusDescription(params map[string]interface{}, status int16, hasStatus bool, progress int16, hasProgress bool) string {
	for _, key := range []string{"status_description", "description", "message", "error"} {
		if raw, ok := params[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(raw))
			if text != "" {
				return text
			}
		}
	}
	if !hasStatus && !hasProgress {
		return ""
	}
	labels := map[int16]string{
		1: "OTA pending push",
		2: "OTA pushed",
		3: "OTA upgrading",
		4: "OTA upgrade succeeded",
		5: "OTA upgrade failed",
		6: "OTA canceled",
	}
	desc := labels[status]
	if desc == "" {
		desc = "OTA status updated"
	}
	if hasProgress {
		desc = fmt.Sprintf("%s, progress %d%%", desc, progress)
	}
	return desc
}

func rdiOTAVersionFromParams(params map[string]interface{}) string {
	for _, key := range []string{"version", "firmware_version", "current_version", "target_version"} {
		if raw, ok := params[key]; ok {
			version := strings.TrimSpace(fmt.Sprint(raw))
			if version != "" {
				return version
			}
		}
	}
	return ""
}
