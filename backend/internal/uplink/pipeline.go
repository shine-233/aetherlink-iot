// pipeline.go is the single uplink pipeline shared by telemetry, attribute and
// event messages:
//
//	resolve device -> processor decode -> direct | gateway fan-out -> per-kind handler
//
// Each message kind supplies only a kindHandler (how to parse one device's
// payload, how to parse a gateway envelope, and what to do for one device).
// Gateway routing, sub-device resolution, depth limiting and diagnostics live
// here once instead of being copied per kind.
package uplink

import (
	"context"
	"fmt"
	"sort"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/processor"

	"github.com/sirupsen/logrus"
)

// maxGatewayDepth bounds sub-gateway recursion. The top gateway's direct
// sub-gateways are depth 1; payloads nested deeper than this are dropped.
const maxGatewayDepth = 5

// kindSpec captures the intentional per-kind differences of the shared
// pipeline that are not expressed by the handler itself.
type kindSpec struct {
	// name is used in lifecycle log lines ("TelemetryUplink started").
	name string
	// gatewayMsgType is the DeviceMessage.Type that selects gateway fan-out.
	gatewayMsgType string
	// dataType selects the processor script.
	dataType processor.DataType
	// recordDecodeDiagnostics records processor failures as uplink failures in
	// diagnostics. Historically only telemetry did this.
	recordDecodeDiagnostics bool
	// gatewayParseErrorLog is the log message for an unparseable gateway envelope.
	gatewayParseErrorLog string
}

// gatewayNode is the kind-neutral shape of a gateway envelope: the gateway's
// own data, its direct sub-devices keyed by sub-device address, and nested
// sub-gateways keyed by address.
type gatewayNode[T any] struct {
	self        *T
	subDevices  map[string]T
	subGateways map[string]*gatewayNode[T]
}

// kindHandler is everything a message kind has to provide.
type kindHandler[T any] interface {
	// parseDirect parses one device's decoded payload (never fails; kinds wrap
	// malformed payloads as raw values).
	parseDirect(device *model.Device, payload []byte) T
	// parseGateway parses a decoded gateway envelope.
	parseGateway(payload []byte) (*gatewayNode[T], error)
	// handleDevice runs the per-device business logic.
	handleDevice(device *model.Device, data T, msg *DeviceMessage)
}

// uplinkBase holds the lifecycle and shared dependencies of one uplink worker.
type uplinkBase struct {
	spec       kindSpec
	processor  processor.DataProcessor
	liveness   *deviceLiveness
	subDevices *subDeviceResolver
	logger     *logrus.Logger
	// loadDevice resolves the top-level device by id (Redis device cache).
	loadDevice func(deviceID string) (*model.Device, error)

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func newUplinkBase(spec kindSpec, proc processor.DataProcessor, heartbeat heartbeatRefresher, logger *logrus.Logger) uplinkBase {
	ctx, cancel := context.WithCancel(context.Background())
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return uplinkBase{
		spec:       spec,
		processor:  proc,
		liveness:   newDeviceLiveness(heartbeat, logger),
		subDevices: sharedSubDeviceResolver(),
		logger:     logger,
		loadDevice: initialize.GetDeviceCacheById,
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
}

// run consumes messageChan until it closes or Stop is called.
func (b *uplinkBase) run(messageChan <-chan *DeviceMessage, process func(*DeviceMessage)) {
	b.logger.Info(b.spec.name + " started")
	go func() {
		defer close(b.done)
		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					b.logger.Info(b.spec.name + " message channel closed")
					return
				}
				process(msg)
			case <-b.ctx.Done():
				b.logger.Info(b.spec.name + " stopped")
				return
			}
		}
	}()
}

// Stop cancels the consume loop.
func (b *uplinkBase) Stop() {
	b.cancel()
}

// Done closes when the consume loop has exited.
func (b *uplinkBase) Done() <-chan struct{} {
	return b.done
}

func (b *uplinkBase) context() context.Context {
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

func (b *uplinkBase) log() *logrus.Logger {
	if b.logger == nil {
		return logrus.StandardLogger()
	}
	return b.logger
}

func (b *uplinkBase) resolveDevice(msg *DeviceMessage) (*model.Device, bool) {
	deviceIDObj, ok := msg.GetMetadata("device_id")
	if !ok {
		b.log().Error("Device ID not found in message metadata")
		return nil, false
	}
	deviceID, ok := deviceIDObj.(string)
	if !ok {
		b.log().Error("Invalid device ID type in metadata")
		return nil, false
	}

	load := b.loadDevice
	if load == nil {
		load = initialize.GetDeviceCacheById
	}
	device, err := load(deviceID)
	if err != nil {
		b.log().WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err,
		}).Error("Failed to get device from cache")
		return nil, false
	}
	return device, true
}

func (b *uplinkBase) decode(device *model.Device, msg *DeviceMessage) ([]byte, bool) {
	if device.DeviceConfigID == nil || *device.DeviceConfigID == "" {
		return msg.Payload, true
	}

	output, err := b.processor.Decode(b.context(), &processor.DecodeInput{
		DeviceConfigID: *device.DeviceConfigID,
		Type:           b.spec.dataType,
		RawData:        msg.Payload,
		Timestamp:      msg.Timestamp,
	})
	if err != nil {
		if b.spec.recordDecodeDiagnostics {
			diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("processor failed: %v", err))
		}
		b.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error("Processor decode failed, terminate processing")
		return nil, false
	}

	if !output.Success {
		if b.spec.recordDecodeDiagnostics {
			errMsg := "processor returned unsuccessful result"
			if output.Error != nil {
				errMsg = fmt.Sprintf("processor returned unsuccessful result: %v", output.Error)
			}
			diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, errMsg)
		}
		b.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     output.Error,
		}).Error("Processor execution failed, terminate processing")
		return nil, false
	}

	return output.Data, true
}

// processUplinkMessage is the whole pipeline for one bus message.
func processUplinkMessage[T any](b *uplinkBase, h kindHandler[T], msg *DeviceMessage) {
	device, ok := b.resolveDevice(msg)
	if !ok {
		return
	}
	payload, ok := b.decode(device, msg)
	if !ok {
		return
	}
	dispatchDecodedPayload(b, h, device, payload, msg)
}

func dispatchDecodedPayload[T any](b *uplinkBase, h kindHandler[T], device *model.Device, payload []byte, msg *DeviceMessage) {
	if msg.Type != b.spec.gatewayMsgType {
		h.handleDevice(device, h.parseDirect(device, payload), msg)
		return
	}

	node, err := h.parseGateway(payload)
	if err != nil {
		diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("gateway message json parse failed: %v", err))
		b.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error(b.spec.gatewayParseErrorLog)
		return
	}
	fanOutGateway(b, h, device, node, msg, 1)
}

// fanOutGateway routes one gateway node in a stable order: the gateway's own
// data, then its direct sub-devices (sorted by address), then nested
// sub-gateways (sorted by address, depth-first). childDepth is the depth that
// this node's sub-gateways will have.
func fanOutGateway[T any](b *uplinkBase, h kindHandler[T], gateway *model.Device, node *gatewayNode[T], msg *DeviceMessage, childDepth int) {
	if node == nil {
		return
	}
	if node.self != nil {
		h.handleDevice(gateway, *node.self, msg)
	}
	if node.subDevices != nil {
		routeSubDevices(b, h, gateway.ID, node.subDevices, msg)
	}
	if node.subGateways != nil {
		routeSubGateways(b, h, gateway.ID, node.subGateways, msg, childDepth)
	}
}

func routeSubDevices[T any](b *uplinkBase, h kindHandler[T], parentID string, data map[string]T, msg *DeviceMessage) {
	if len(data) == 0 {
		return
	}
	addrs := sortedKeys(data)
	devices, err := b.subDevices.resolve(parentID, addrs)
	if err != nil {
		b.log().WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub devices")
		return
	}
	for _, addr := range addrs {
		subDevice, ok := devices[addr]
		if !ok {
			b.log().WithFields(logrus.Fields{
				"parent_id":   parentID,
				"device_addr": addr,
			}).Warn("Sub device not found")
			continue
		}
		h.handleDevice(subDevice, data[addr], msg)
	}
}

func routeSubGateways[T any](b *uplinkBase, h kindHandler[T], parentID string, data map[string]*gatewayNode[T], msg *DeviceMessage, depth int) {
	if depth > maxGatewayDepth {
		b.log().Warn(fmt.Sprintf("Maximum gateway depth (%d) exceeded", maxGatewayDepth))
		return
	}
	if len(data) == 0 {
		return
	}
	addrs := sortedKeys(data)
	gateways, err := b.subDevices.resolve(parentID, addrs)
	if err != nil {
		b.log().WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub gateways")
		return
	}
	for _, addr := range addrs {
		subGateway, ok := gateways[addr]
		if !ok {
			b.log().WithFields(logrus.Fields{
				"parent_id":    parentID,
				"gateway_addr": addr,
			}).Warn("Sub gateway not found")
			continue
		}
		fanOutGateway(b, h, subGateway, data[addr], msg, depth+1)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// gatewayPublishNode converts the telemetry/attribute gateway envelope.
// Values are passed through as already-decoded maps (the old code re-marshaled
// each map to JSON and parsed it again, which produced the same map).
func gatewayPublishNode(msg *model.GatewayPublish) *gatewayNode[map[string]interface{}] {
	if msg == nil {
		return nil
	}
	node := &gatewayNode[map[string]interface{}]{self: msg.GatewayData}
	if msg.SubDeviceData != nil {
		node.subDevices = *msg.SubDeviceData
		if node.subDevices == nil {
			node.subDevices = map[string]map[string]interface{}{}
		}
	}
	if msg.SubGatewayData != nil {
		node.subGateways = make(map[string]*gatewayNode[map[string]interface{}], len(*msg.SubGatewayData))
		for addr, child := range *msg.SubGatewayData {
			node.subGateways[addr] = gatewayPublishNode(child)
		}
	}
	return node
}

// gatewayEventNode converts the event gateway envelope.
func gatewayEventNode(msg *model.GatewayCommandPulish) *gatewayNode[model.EventInfo] {
	if msg == nil {
		return nil
	}
	node := &gatewayNode[model.EventInfo]{self: msg.GatewayData}
	if msg.SubDeviceData != nil {
		node.subDevices = *msg.SubDeviceData
		if node.subDevices == nil {
			node.subDevices = map[string]model.EventInfo{}
		}
	}
	if msg.SubGatewayData != nil {
		node.subGateways = make(map[string]*gatewayNode[model.EventInfo], len(*msg.SubGatewayData))
		for addr, child := range *msg.SubGatewayData {
			node.subGateways[addr] = gatewayEventNode(child)
		}
	}
	return node
}
