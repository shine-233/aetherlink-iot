package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

func TestValidateSceneAutomationEventParamTriggerValueRejectsInvalidMatchConfigs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		triggerValue string
		wantMessage  string
	}{
		{
			name:         "empty conditions",
			triggerValue: `{"match_mode":"field","conditions":[]}`,
			wantMessage:  "event trigger_value must contain at least one condition",
		},
		{
			name:         "unknown operator",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"contains","value":"hot"}]}`,
			wantMessage:  "event trigger_value operator [contains] is not supported",
		},
		{
			name:         "between requires ordered numeric values",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"between","value":[20,10]}]}`,
			wantMessage:  "event trigger_value between operator requires two ordered numeric values",
		},
		{
			name:         "in requires non-empty list",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"code","operator":"in","value":[]}]}`,
			wantMessage:  "event trigger_value in operator requires a non-empty list",
		},
		{
			name:         "exists requires boolean",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"online","operator":"exists","value":"true"}]}`,
			wantMessage:  "event trigger_value exists operator requires a boolean value",
		},
		{
			name:         "numeric operator requires number",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":">=","value":"warn"}]}`,
			wantMessage:  "event trigger_value numeric operator requires a number",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateSceneAutomationEventParamTriggerValue(model.Condition{
				TriggerParamType: pureHelperStringPtr("event"),
				TriggerValue:     &tc.triggerValue,
			})

			assertErrcodeError(t, err, tc.name, errcode.CodeParamError, tc.wantMessage)
		})
	}
}

func TestValidateSceneAutomationEventParamTriggerValueAcceptsStructuredMatchConfig(t *testing.T) {
	t.Parallel()

	triggerValue := `{
		"match_mode":"field",
		"conditions":[
			{"field":"level","operator":">=","value":80},
			{"field":"code","operator":"in","value":["A","B"]},
			{"field":"online","operator":"exists","value":true},
			{"field":"temperature","operator":"between","value":[10,20]}
		]
	}`

	err := validateSceneAutomationEventParamTriggerValue(model.Condition{
		TriggerParamType: pureHelperStringPtr("event"),
		TriggerValue:     &triggerValue,
	})
	if err != nil {
		t.Fatalf("valid event param match config should pass validation, got %v", err)
	}
}
