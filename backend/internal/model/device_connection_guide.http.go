package model

import "time"

type DeviceConnectionGuideResp struct {
	DeviceID       string                          `json:"device_id"`
	EvaluatedAt    time.Time                       `json:"evaluated_at"`
	Access         DeviceConnectionGuideAccess     `json:"access"`
	Readiness      DeviceConnectionGuideReadiness  `json:"readiness"`
	LastError      *DeviceConnectionGuideLastError `json:"last_connection_error,omitempty"`
	TwinSummary    *DeviceTwinSummary              `json:"twin_summary,omitempty"`
	CommandSummary *DeviceConnectionGuideCommand   `json:"command_summary,omitempty"`
	NextSteps      []DeviceConnectionGuideStep     `json:"next_steps"`
	PartialResults []DeviceConnectionGuideWarning  `json:"partial_results,omitempty"`
}

type DeviceConnectionGuideAccess struct {
	Protocol          string                         `json:"protocol"`
	CredentialMode    string                         `json:"credential_mode,omitempty"`
	ConnectionInfo    interface{}                    `json:"connection_info,omitempty"`
	ConnectionProfile *DeviceConnectionGuideProfile  `json:"connection_profile,omitempty"`
	CredentialForm    interface{}                    `json:"credential_form,omitempty"`
	TLS               DeviceConnectionGuideTLSHint   `json:"tls"`
	HTTPHint          *DeviceConnectionGuideHTTPHint `json:"http_hint,omitempty"`
}

type DeviceConnectionGuideProfile struct {
	Protocol           string `json:"protocol,omitempty"`
	Endpoint           string `json:"endpoint,omitempty"`
	Host               string `json:"host,omitempty"`
	Port               string `json:"port,omitempty"`
	TLSEnabled         bool   `json:"tls_enabled,omitempty"`
	CredentialMode     string `json:"credential_mode,omitempty"`
	CredentialRequired bool   `json:"credential_required"`
	DeviceType         string `json:"device_type,omitempty"`
	DeviceNumber       string `json:"device_number,omitempty"`
	ClientID           string `json:"client_id,omitempty"`
	Username           string `json:"username,omitempty"`
	TelemetryTopic     string `json:"telemetry_topic,omitempty"`
	CommandTopic       string `json:"command_topic,omitempty"`
	TestPayload        string `json:"test_payload,omitempty"`
	SamplePayload      string `json:"sample_payload,omitempty"`
	HTTPAddress        string `json:"http_address,omitempty"`
	SubTopicPrefix     string `json:"sub_topic_prefix,omitempty"`
}

type DeviceConnectionGuideTLSHint struct {
	Enabled            bool   `json:"enabled"`
	Broker             string `json:"broker,omitempty"`
	CertificateHint    string `json:"certificate_hint"`
	AdvertisedToDevice string `json:"advertised_to_device"`
}

type DeviceConnectionGuideHTTPHint struct {
	Available bool   `json:"available"`
	Summary   string `json:"summary"`
}

type DeviceConnectionGuideReadiness struct {
	Level             string     `json:"level"`
	Code              string     `json:"code"`
	Summary           string     `json:"summary"`
	Online            bool       `json:"online"`
	Ready             bool       `json:"ready"`
	LatestTelemetryAt *time.Time `json:"latest_telemetry_at,omitempty"`
	NextActions       []string   `json:"next_actions"`
	Evidence          []string   `json:"evidence,omitempty"`
}

type DeviceConnectionGuideLastError struct {
	Code     string   `json:"code"`
	Summary  string   `json:"summary"`
	Evidence []string `json:"evidence,omitempty"`
}

type DeviceConnectionGuideCommand struct {
	Level           string   `json:"level"`
	Code            string   `json:"code"`
	Summary         string   `json:"summary"`
	LatestStatus    string   `json:"latest_status,omitempty"`
	LatestMessageID string   `json:"latest_message_id,omitempty"`
	NextActions     []string `json:"next_actions,omitempty"`
}

type DeviceConnectionGuideStep struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type DeviceConnectionGuideWarning struct {
	Component string `json:"component"`
	Reason    string `json:"reason"`
}
