// 文件用途：payload_schema_enforce.go 纯决策函数的单元测试。
// 覆盖:accept / warn / reject 三类 outcome,以及各字段类型/必填/范围/枚举/正则/严格模式分支。
// 这些用例只验证“可离线推演”的一半;broker 真实拦截(接入 OnMsgArrivedWrapper)需运行时验证,不在此覆盖。
package aetherlink

import "testing"

func f64(v float64) *float64 { return &v }
func strp(v string) *string  { return &v }

func TestDecidePayloadSchemaEnforcement_AcceptValid(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Required: true, Min: f64(-40), Max: f64(125)},
			{Name: "mode", Type: PayloadSchemaFieldTypeString, Enum: []string{"auto", "manual"}},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":21.5,"mode":"auto"}`))
	if d.Outcome != PayloadSchemaAccept {
		t.Fatalf("expected accept, got %s (errors=%v)", d.Outcome, d.Errors)
	}
	if d.Checkedn != 2 {
		t.Fatalf("expected 2 checked fields, got %d", d.Checkedn)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectInvalidJSON(t *testing.T) {
	d := DecidePayloadSchemaEnforcement(PayloadSchemaEnforcement{}, []byte(`not json`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for invalid json, got %s", d.Outcome)
	}
	if len(d.Errors) == 0 {
		t.Fatal("expected an error for invalid json")
	}
}

func TestDecidePayloadSchemaEnforcement_RejectsNullRoot(t *testing.T) {
	d := DecidePayloadSchemaEnforcement(PayloadSchemaEnforcement{}, []byte(`null`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for null root, got %s", d.Outcome)
	}
	if len(d.Errors) != 1 || d.Errors[0] != "payload is not a valid JSON object" {
		t.Fatalf("expected the non-object diagnostic, got %v", d.Errors)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectsUnsupportedFieldType(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "temp", Type: PayloadSchemaFieldType("integer")},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":1}`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for unsupported field type, got %s", d.Outcome)
	}
	if len(d.Errors) != 1 || d.Errors[0] != `field "temp" has unsupported type "integer"` {
		t.Fatalf("expected unsupported-type diagnostic, got %v", d.Errors)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectMissingRequired(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "id", Type: PayloadSchemaFieldTypeString, Required: true},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"other":1}`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for missing required, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_OptionalMissingIsAccept(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "note", Type: PayloadSchemaFieldTypeString},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{}`))
	if d.Outcome != PayloadSchemaAccept {
		t.Fatalf("expected accept for missing optional, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectNumberOutOfRange(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Min: f64(0), Max: f64(100)},
		},
	}
	if d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":150}`)); d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject above max, got %s", d.Outcome)
	}
	if d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":-5}`)); d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject below min, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectWrongType(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "temp", Type: PayloadSchemaFieldTypeNumber},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":"hot"}`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for wrong type, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectEnumViolation(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "mode", Type: PayloadSchemaFieldTypeString, Enum: []string{"auto", "manual"}},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"mode":"turbo"}`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for enum violation, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectPatternMismatch(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "sn", Type: PayloadSchemaFieldTypeString, Pattern: strp(`^SN-[0-9]+$`)},
		},
	}
	if d := DecidePayloadSchemaEnforcement(enf, []byte(`{"sn":"bad"}`)); d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for pattern mismatch, got %s", d.Outcome)
	}
	if d := DecidePayloadSchemaEnforcement(enf, []byte(`{"sn":"SN-42"}`)); d.Outcome != PayloadSchemaAccept {
		t.Fatalf("expected accept for pattern match, got %s (errors=%v)", d.Outcome, d.Errors)
	}
}

func TestDecidePayloadSchemaEnforcement_RejectInvalidPattern(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "sn", Type: PayloadSchemaFieldTypeString, Pattern: strp(`([`)},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"sn":"x"}`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for invalid pattern, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_BooleanObjectArray(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "on", Type: PayloadSchemaFieldTypeBoolean},
			{Name: "meta", Type: PayloadSchemaFieldTypeObject},
			{Name: "tags", Type: PayloadSchemaFieldTypeArray},
		},
	}
	if d := DecidePayloadSchemaEnforcement(enf, []byte(`{"on":true,"meta":{"a":1},"tags":[1,2]}`)); d.Outcome != PayloadSchemaAccept {
		t.Fatalf("expected accept, got %s (errors=%v)", d.Outcome, d.Errors)
	}
	if d := DecidePayloadSchemaEnforcement(enf, []byte(`{"on":"yes","meta":1,"tags":"nope"}`)); d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for wrong types, got %s", d.Outcome)
	}
}

func TestDecidePayloadSchemaEnforcement_StrictRejectsUnknown(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Strict: true,
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "temp", Type: PayloadSchemaFieldTypeNumber},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":20,"extra":1}`))
	if d.Outcome != PayloadSchemaReject {
		t.Fatalf("expected reject for unknown key in strict mode, got %s", d.Outcome)
	}
	if len(d.UnknownKeys) != 1 || d.UnknownKeys[0] != "extra" {
		t.Fatalf("expected unknown key 'extra', got %v", d.UnknownKeys)
	}
}

func TestDecidePayloadSchemaEnforcement_NonStrictWarnsUnknown(t *testing.T) {
	enf := PayloadSchemaEnforcement{
		Strict: false,
		Fields: []PayloadSchemaFieldConstraint{
			{Name: "temp", Type: PayloadSchemaFieldTypeNumber},
		},
	}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"temp":20,"extra":1}`))
	if d.Outcome != PayloadSchemaWarn {
		t.Fatalf("expected warn for unknown key in non-strict mode, got %s", d.Outcome)
	}
	if len(d.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", d.Warnings)
	}
}

func TestDecidePayloadSchemaEnforcement_UnknownKeysSorted(t *testing.T) {
	enf := PayloadSchemaEnforcement{Fields: nil}
	d := DecidePayloadSchemaEnforcement(enf, []byte(`{"z":1,"a":2,"m":3}`))
	want := []string{"a", "m", "z"}
	if len(d.UnknownKeys) != len(want) {
		t.Fatalf("expected %v, got %v", want, d.UnknownKeys)
	}
	for i := range want {
		if d.UnknownKeys[i] != want[i] {
			t.Fatalf("expected sorted %v, got %v", want, d.UnknownKeys)
		}
	}
}
