package service

import (
	"context"
	"strings"
	"testing"
)

func TestCloudRuleNodeValidators(t *testing.T) {
	// 1. AWS SQS
	if err := validateAWSSQSConfig(nil); err == nil {
		t.Fatal("expected error on nil config")
	}
	if err := validateAWSSQSConfig(map[string]any{"queue_url": ""}); err == nil {
		t.Fatal("expected error on empty queue_url")
	}
	if err := validateAWSSQSConfig(map[string]any{"queue_url": "https://sqs.us-east-1.amazonaws.com/123/my-queue"}); err == nil {
		t.Fatal("expected error on missing region")
	}
	if err := validateAWSSQSConfig(map[string]any{
		"queue_url": "https://sqs.us-east-1.amazonaws.com/123/my-queue",
		"region":    "us-east-1",
	}); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}

	// 2. AWS SNS
	if err := validateAWSSNSConfig(nil); err == nil {
		t.Fatal("expected error on nil config")
	}
	if err := validateAWSSNSConfig(map[string]any{"topic_arn": ""}); err == nil {
		t.Fatal("expected error on empty topic_arn")
	}
	if err := validateAWSSNSConfig(map[string]any{"topic_arn": "arn:aws:sns:us-east-1:123:my-topic"}); err == nil {
		t.Fatal("expected error on missing region")
	}
	if err := validateAWSSNSConfig(map[string]any{
		"topic_arn": "arn:aws:sns:us-east-1:123:my-topic",
		"region":    "us-east-1",
	}); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}

	// 3. Azure IoT Hub
	if err := validateAzureIoTHubConfig(nil); err == nil {
		t.Fatal("expected error on nil config")
	}
	if err := validateAzureIoTHubConfig(map[string]any{"hub_name": ""}); err == nil {
		t.Fatal("expected error on empty hub_name")
	}
	if err := validateAzureIoTHubConfig(map[string]any{"hub_name": "my-iothub"}); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
}

func TestRuleChainExternalAWSSQS(t *testing.T) {
	origProducer := ruleChainAWSSQSProducer
	defer func() { ruleChainAWSSQSProducer = origProducer }()

	var captured []string
	ruleChainAWSSQSProducer = func(_ context.Context, queueURL, region string, payload []byte) error {
		captured = append(captured, queueURL+"|"+region+"|"+string(payload))
		return nil
	}

	node := d1Node("sqs1", RuleChainExternalAWSSQS, map[string]any{
		"queue_url": "https://sqs.us-east-1.amazonaws.com/123/my-queue",
		"region":    "us-east-1",
	})
	rcc := d1RCC()
	res, err := ruleChainExternalAWSSQS(context.Background(), node, rcc, map[string]any{"temp": 25.4}, map[string]any{"dev": "d1"})
	if err != nil || !res.pass {
		t.Fatalf("sqs dispatch failed: %v", err)
	}
	if len(captured) != 1 || !strings.Contains(captured[0], "my-queue") || !strings.Contains(captured[0], "us-east-1") {
		t.Fatalf("sqs capture mismatch: %+v", captured)
	}

	// 未装配 → noop 放行
	ruleChainAWSSQSProducer = nil
	res, err = ruleChainExternalAWSSQS(context.Background(), node, rcc, map[string]any{}, map[string]any{})
	if err != nil || !res.pass {
		t.Fatalf("unwired sqs should noop pass: %v", err)
	}
}

func TestRuleChainExternalAWSSNS(t *testing.T) {
	origProducer := ruleChainAWSSNSProducer
	defer func() { ruleChainAWSSNSProducer = origProducer }()

	var captured []string
	ruleChainAWSSNSProducer = func(_ context.Context, topicARN, region string, payload []byte) error {
		captured = append(captured, topicARN+"|"+region+"|"+string(payload))
		return nil
	}

	node := d1Node("sns1", RuleChainExternalAWSSNS, map[string]any{
		"topic_arn": "arn:aws:sns:us-east-1:123:telemetry",
		"region":    "us-east-1",
	})
	rcc := d1RCC()
	res, err := ruleChainExternalAWSSNS(context.Background(), node, rcc, map[string]any{"alarm": "critical"}, map[string]any{})
	if err != nil || !res.pass {
		t.Fatalf("sns dispatch failed: %v", err)
	}
	if len(captured) != 1 || !strings.Contains(captured[0], "telemetry") {
		t.Fatalf("sns capture mismatch: %+v", captured)
	}

	// 未装配 → noop 放行
	ruleChainAWSSNSProducer = nil
	res, err = ruleChainExternalAWSSNS(context.Background(), node, rcc, map[string]any{}, map[string]any{})
	if err != nil || !res.pass {
		t.Fatalf("unwired sns should noop pass: %v", err)
	}
}

func TestRuleChainExternalAzureIoTHub(t *testing.T) {
	origProducer := ruleChainAzureIoTHubProducer
	defer func() { ruleChainAzureIoTHubProducer = origProducer }()

	var captured []string
	ruleChainAzureIoTHubProducer = func(_ context.Context, hubName, deviceID string, payload []byte) error {
		captured = append(captured, hubName+"|"+deviceID+"|"+string(payload))
		return nil
	}

	node := d1Node("az1", RuleChainExternalAzureIoTHub, map[string]any{
		"hub_name":  "contoso-iothub",
		"device_id": "sensor-99",
	})
	rcc := d1RCC()
	res, err := ruleChainExternalAzureIoTHub(context.Background(), node, rcc, map[string]any{"pressure": 101.3}, map[string]any{})
	if err != nil || !res.pass {
		t.Fatalf("azure iothub dispatch failed: %v", err)
	}
	if len(captured) != 1 || !strings.Contains(captured[0], "contoso-iothub|sensor-99") {
		t.Fatalf("azure iothub capture mismatch: %+v", captured)
	}

	// 未装配 → noop 放行
	ruleChainAzureIoTHubProducer = nil
	res, err = ruleChainExternalAzureIoTHub(context.Background(), node, rcc, map[string]any{}, map[string]any{})
	if err != nil || !res.pass {
		t.Fatalf("unwired azure iothub should noop pass: %v", err)
	}
}

func TestRuleChainNodeSpecListContainsCloudNodes(t *testing.T) {
	specs := RuleChainNodeSpecList()
	foundSQS, foundSNS, foundAzure := false, false, false
	for _, spec := range specs {
		if spec.Type == RuleChainExternalAWSSQS {
			foundSQS = true
		}
		if spec.Type == RuleChainExternalAWSSNS {
			foundSNS = true
		}
		if spec.Type == RuleChainExternalAzureIoTHub {
			foundAzure = true
		}
	}
	if !foundSQS || !foundSNS || !foundAzure {
		t.Fatalf("expected cloud nodes in spec list: sqs=%v, sns=%v, azure=%v", foundSQS, foundSNS, foundAzure)
	}
}
