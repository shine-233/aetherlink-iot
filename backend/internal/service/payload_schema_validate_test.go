package service

import (
	"testing"

	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
)

func floatPtr(v float64) *float64 { return &v }

func TestValidatePayloadRequiresClaims(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	if _, err := svc.ValidatePayload(&model.ValidatePayloadReq{}, nil); err == nil {
		t.Fatal("expected error when claims are nil")
	}
}

func TestValidatePayloadRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	res, err := svc.ValidatePayload(&model.ValidatePayloadReq{
		Fields:        []model.PayloadSchemaField{{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber}},
		SamplePayload: "not-json",
	}, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatal("expected invalid result for non-JSON payload")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected an error message for non-JSON payload")
	}
}

func TestValidatePayloadPassesWhenSchemaMatches(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	res, err := svc.ValidatePayload(&model.ValidatePayloadReq{
		Fields: []model.PayloadSchemaField{
			{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber, Required: true, Min: floatPtr(0), Max: floatPtr(100)},
			{Name: "mode", Type: model.PayloadSchemaFieldTypeString, Enum: []string{"auto", "manual"}},
			{Name: "online", Type: model.PayloadSchemaFieldTypeBoolean},
		},
		SamplePayload: `{"temp":25.5,"mode":"auto","online":true}`,
	}, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid result, got errors: %v", res.Errors)
	}
	if res.CheckedFields != 3 {
		t.Fatalf("expected 3 checked fields, got %d", res.CheckedFields)
	}
}

func TestValidatePayloadFlagsMissingRequiredAndRangeAndType(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	res, err := svc.ValidatePayload(&model.ValidatePayloadReq{
		Fields: []model.PayloadSchemaField{
			{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber, Required: true, Max: floatPtr(50)},
			{Name: "label", Type: model.PayloadSchemaFieldTypeString, Required: true},
		},
		SamplePayload: `{"temp":80,"label":123}`,
	}, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatal("expected invalid result")
	}
	// expect: temp above max, label wrong type
	if len(res.Errors) < 2 {
		t.Fatalf("expected at least 2 errors, got %v", res.Errors)
	}
}

func TestValidatePayloadStrictModeRejectsUnknownKeys(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	req := &model.ValidatePayloadReq{
		Strict:        true,
		Fields:        []model.PayloadSchemaField{{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber}},
		SamplePayload: `{"temp":10,"extra":"x"}`,
	}
	res, err := svc.ValidatePayload(req, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatal("expected invalid result in strict mode with unknown key")
	}
	if len(res.UnknownKeys) != 1 || res.UnknownKeys[0] != "extra" {
		t.Fatalf("expected unknown key 'extra', got %v", res.UnknownKeys)
	}
}

func TestValidatePayloadNonStrictWarnsUnknownKeys(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	res, err := svc.ValidatePayload(&model.ValidatePayloadReq{
		Fields:        []model.PayloadSchemaField{{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber}},
		SamplePayload: `{"temp":10,"extra":"x"}`,
	}, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid result in non-strict mode, got errors: %v", res.Errors)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected a warning for the undeclared key")
	}
}

func TestValidatePayloadPatternMismatch(t *testing.T) {
	t.Parallel()

	svc := &PayloadSchema{}
	res, err := svc.ValidatePayload(&model.ValidatePayloadReq{
		Fields: []model.PayloadSchemaField{
			{Name: "sn", Type: model.PayloadSchemaFieldTypeString, Pattern: strPtr(`^SN-[0-9]+$`)},
		},
		SamplePayload: `{"sn":"bad"}`,
	}, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatal("expected invalid result for pattern mismatch")
	}
}
