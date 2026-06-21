package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/tyandl/homelink/pkg/yolink/types"
)

// mqttRequestTimeout is how long a request waits for its response before it is assumed lost.
const mqttRequestTimeout = 30 * time.Second

// MqttMessage is a received MQTT message, decoupled from the underlying paho type.
type MqttMessage struct {
	Topic   string
	Payload []byte
}

// mqttResult is the outcome of an MQTT request: a decoded response packet, or an error
// (timeout, publish failure, or connection loss).
type mqttResult struct {
	Response basicUploadDataPacket
	Err      error
}

// inflight is a single request awaiting its response.
type inflight struct {
	deviceId string
	result   chan mqttResult
	timer    *time.Timer
}

// deviceEntry tracks the response subscription shared by a device's in-flight requests. It
// exists only while at least one request is in flight (or pending unsubscribe).
type deviceEntry struct {
	refCount int
	ready    chan struct{} // closed once the response subscription is established
}

// MqttClient wraps a paho MQTT client and layers request/response correlation on top: it
// publishes commands to a device's request topic, subscribes to its response topic on
// demand, matches responses to requests by msgid, and tears the subscription down when the
// device has no requests in flight. All of this bookkeeping is internal.
type MqttClient struct {
	paho   paho.Client
	homeId string

	// mu guards pending and devices.
	mu      sync.Mutex
	pending map[types.MessageId]*inflight
	devices map[string]*deviceEntry

	// subMu serializes paho Subscribe/Unsubscribe calls so a stale unsubscribe can't race
	// a fresh subscribe for the same topic.
	subMu sync.Mutex

	// subsMu guards subscriptions: the set of active report subscriptions, so Disconnect and
	// connection loss can release them all.
	subsMu        sync.Mutex
	subscriptions map[*reportSub]struct{}
}

func newMqttClient() *MqttClient {
	return &MqttClient{
		pending:       make(map[types.MessageId]*inflight),
		devices:       make(map[string]*deviceEntry),
		subscriptions: make(map[*reportSub]struct{}),
	}
}

// reportSub tracks one active report subscription so it can be released individually (via the
// stop func returned by subscribeReports) or in bulk on Disconnect / connection loss.
type reportSub struct {
	topic string
	done  chan struct{}
	once  sync.Once
}

// close releases the subscription's forwarder goroutine and Report channel. Idempotent.
func (sub *reportSub) close() {
	sub.once.Do(func() { close(sub.done) })
}

func (client *MqttClient) registerSubscription(sub *reportSub) {
	client.subsMu.Lock()
	client.subscriptions[sub] = struct{}{}
	client.subsMu.Unlock()
}

func (client *MqttClient) removeSubscription(sub *reportSub) {
	client.subsMu.Lock()
	delete(client.subscriptions, sub)
	client.subsMu.Unlock()
}

// shutdown fails all in-flight calls and releases every active report subscription. It is
// invoked on deliberate Disconnect and on unexpected connection loss.
func (client *MqttClient) shutdown(cause error) {
	slog.Debug("mqtt shutdown", "cause", cause)
	client.failAll(cause)

	client.subsMu.Lock()
	subs := make([]*reportSub, 0, len(client.subscriptions))
	for sub := range client.subscriptions {
		subs = append(subs, sub)
	}
	client.subscriptions = make(map[*reportSub]struct{})
	client.subsMu.Unlock()

	for _, sub := range subs {
		sub.close()
	}
}

// connect establishes the broker connection, authenticating with token as the username.
// homeId is used to build the per-device request/response topics.
func (client *MqttClient) connect(broker string, token string, homeId string, onConnectionLost func(error)) error {
	client.homeId = homeId

	opts := paho.NewClientOptions().
		AddBroker(broker).
		SetUsername(token).
		SetPassword("").
		SetKeepAlive(60 * time.Second).
		SetAutoReconnect(false).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			client.shutdown(err)
			if onConnectionLost != nil {
				onConnectionLost(err)
			}
		})

	pahoClient := paho.NewClient(opts)
	connectToken := pahoClient.Connect()
	if !connectToken.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if err := connectToken.Error(); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}
	client.paho = pahoClient
	slog.Debug("mqtt connected", "broker", broker)
	return nil
}

// call publishes bddp to the device's request topic and returns a buffered channel that
// receives exactly one mqttResult — the matching response, or an error if none arrives
// within mqttRequestTimeout. It returns immediately; the publish and subscribe happen on a
// background goroutine, so many requests across many devices can be in flight at once.
func (client *MqttClient) call(deviceId string, bddp *basicDownloadDataPacket) <-chan mqttResult {
	result := make(chan mqttResult, 1)

	if client.paho == nil {
		result <- mqttResult{Err: fmt.Errorf("mqtt client is not connected")}
		return result
	}
	if bddp.MsgId == "" {
		bddp.MsgId = types.NewMessageId()
	}
	msgId := bddp.MsgId
	slog.Debug("mqtt publish request", "device", deviceId, "msgid", msgId, "method", bddp.Method)

	payload, err := json.Marshal(bddp)
	if err != nil {
		result <- mqttResult{Err: err}
		return result
	}

	entry, first := client.register(deviceId, msgId, result)

	go func() {
		// Make sure the response subscription is live before publishing.
		if first {
			if err := client.subscribeDevice(deviceId, entry); err != nil {
				client.complete(msgId, mqttResult{Err: err})
				return
			}
		} else {
			<-entry.ready
		}

		token := client.paho.Publish(client.requestTopic(deviceId), 0, false, payload)
		token.Wait()
		if err := token.Error(); err != nil {
			client.complete(msgId, mqttResult{Err: fmt.Errorf("mqtt publish: %w", err)})
		}
	}()

	return result
}

// register records an in-flight request and arms its timeout. It returns the device's
// subscription entry and whether this is the first in-flight request for the device (in
// which case the caller must establish the subscription).
func (client *MqttClient) register(deviceId string, msgId types.MessageId, result chan mqttResult) (*deviceEntry, bool) {
	client.mu.Lock()
	defer client.mu.Unlock()

	entry, ok := client.devices[deviceId]
	first := !ok
	if first {
		entry = &deviceEntry{ready: make(chan struct{})}
		client.devices[deviceId] = entry
	}
	entry.refCount++

	request := &inflight{deviceId: deviceId, result: result}
	request.timer = time.AfterFunc(mqttRequestTimeout, func() {
		client.complete(msgId, mqttResult{Err: fmt.Errorf("mqtt request timed out after %s", mqttRequestTimeout)})
	})
	client.pending[msgId] = request
	return entry, first
}

// complete delivers result to the request identified by msgId and cleans it up. It is
// idempotent: a timeout racing a response (or a duplicate response) finds no pending entry
// and is ignored, so exactly one result is ever delivered.
func (client *MqttClient) complete(msgId types.MessageId, result mqttResult) {
	client.mu.Lock()
	request, ok := client.pending[msgId]
	if !ok {
		client.mu.Unlock()
		return
	}
	delete(client.pending, msgId)
	request.timer.Stop()
	client.release(request.deviceId)
	client.mu.Unlock()

	request.result <- result // buffered (size 1): never blocks
}

// release decrements the in-flight count for a device, scheduling an unsubscribe once it
// reaches zero. Caller must hold client.mu.
func (client *MqttClient) release(deviceId string) {
	entry, ok := client.devices[deviceId]
	if !ok {
		return
	}
	entry.refCount--
	if entry.refCount == 0 {
		go client.unsubscribeDevice(deviceId)
	}
}

// handleResponse routes an incoming response to the matching request by its msgid.
func (client *MqttClient) handleResponse(payload []byte) {
	var budp basicUploadDataPacket
	if err := json.Unmarshal(payload, &budp); err != nil {
		return // can't correlate an unparseable response
	}
	msgId, _ := budp.MsgId.(string)
	if msgId == "" {
		return
	}
	slog.Debug("mqtt response received", "msgid", msgId)
	client.complete(msgId, mqttResult{Response: budp})
}

// failAll completes every in-flight request with a connection-loss error and clears all
// subscription state. Invoked when the broker connection drops.
func (client *MqttClient) failAll(cause error) {
	client.mu.Lock()
	pending := client.pending
	client.pending = make(map[types.MessageId]*inflight)
	client.devices = make(map[string]*deviceEntry)
	client.mu.Unlock()

	err := fmt.Errorf("mqtt connection lost: %w", cause)
	for _, request := range pending {
		request.timer.Stop()
		request.result <- mqttResult{Err: err}
	}
}

// subscribeDevice subscribes to the device's response topic and closes entry.ready so
// callers waiting on the subscription can proceed. Serialized against unsubscribeDevice.
func (client *MqttClient) subscribeDevice(deviceId string, entry *deviceEntry) error {
	client.subMu.Lock()
	token := client.paho.Subscribe(client.responseTopic(deviceId), 0, func(_ paho.Client, msg paho.Message) {
		client.handleResponse(msg.Payload())
	})
	token.Wait()
	err := token.Error()
	client.subMu.Unlock()

	close(entry.ready)
	if err != nil {
		return fmt.Errorf("mqtt subscribe: %w", err)
	}
	slog.Debug("mqtt response-subscribed", "device", deviceId)
	return nil
}

// unsubscribeDevice removes the device's response subscription once no requests remain in
// flight. The refCount is re-checked under both locks so a request that arrived after the
// unsubscribe was scheduled keeps its subscription.
func (client *MqttClient) unsubscribeDevice(deviceId string) {
	client.subMu.Lock()
	defer client.subMu.Unlock()

	client.mu.Lock()
	entry, ok := client.devices[deviceId]
	if !ok || entry.refCount != 0 {
		client.mu.Unlock()
		return
	}
	delete(client.devices, deviceId)
	client.mu.Unlock()

	token := client.paho.Unsubscribe(client.responseTopic(deviceId))
	token.Wait()
	slog.Debug("mqtt response-unsubscribed", "device", deviceId)
}

func (client *MqttClient) requestTopic(deviceId string) string {
	return fmt.Sprintf("yl-home/%s/%s/request", client.homeId, deviceId)
}

func (client *MqttClient) responseTopic(deviceId string) string {
	return fmt.Sprintf("yl-home/%s/%s/response", client.homeId, deviceId)
}

func (client *MqttClient) reportTopic(deviceId string) string {
	return fmt.Sprintf("yl-home/%s/%s/report", client.homeId, deviceId)
}

// Subscribe registers handler for every message published to topic. Used for the report
// event stream, independent of the request/response machinery above.
func (client *MqttClient) Subscribe(topic string, handler func(MqttMessage)) error {
	if client.paho == nil {
		return fmt.Errorf("mqtt client is not connected")
	}
	subscribeToken := client.paho.Subscribe(topic, 0, func(_ paho.Client, msg paho.Message) {
		handler(MqttMessage{Topic: msg.Topic(), Payload: msg.Payload()})
	})
	subscribeToken.Wait()
	if err := subscribeToken.Error(); err != nil {
		return fmt.Errorf("mqtt subscribe: %w", err)
	}
	slog.Info("mqtt subscribed", "topic", topic)
	return nil
}

// unsubscribe removes a topic subscription, waiting for the broker to acknowledge.
func (client *MqttClient) unsubscribe(topic string) {
	if client.paho == nil {
		return
	}
	token := client.paho.Unsubscribe(topic)
	token.Wait()
	slog.Info("mqtt unsubscribed", "topic", topic)
}

// Disconnect closes the connection, waiting up to 250ms for in-flight work.
func (client *MqttClient) Disconnect() {
	if client == nil || client.paho == nil {
		return
	}
	// Release report subscriptions (closes their channels, ends forwarders) and fail any
	// in-flight calls before dropping the connection.
	client.shutdown(fmt.Errorf("mqtt client disconnected"))
	client.paho.Disconnect(250)
}

// Report is a parsed device event received over MQTT.
type Report struct {
	Device DeviceController // the reporting device, or nil if it is not in the account
	Event  string           // event name, e.g. "DoorSensor.Alert"
	Data   any              // typed event payload, or the raw JSON string if unrecognized
}

// subscribeReports subscribes to topic and delivers each message, parsed by parser.ParseEvent,
// on the returned channel. The returned stop function unsubscribes from topic and ends
// delivery, closing the channel; it is safe to call more than once.
func subscribeReports(client *MqttClient, topic string, parser DeviceController) (<-chan Report, func(), error) {
	sub := &reportSub{topic: topic, done: make(chan struct{})}
	incoming := make(chan Report, 32)

	err := client.Subscribe(topic, func(msg MqttMessage) {
		report, ok := parser.ParseEvent(msg)
		if !ok {
			return
		}
		select {
		case incoming <- report:
		case <-sub.done: // receiver stopped listening; drop
		}
	})
	if err != nil {
		return nil, nil, err
	}
	client.registerSubscription(sub)

	// A single forwarder owns out, so it is the only goroutine that ever closes it — the
	// paho callback above only ever writes to incoming.
	out := make(chan Report)
	go func() {
		defer close(out)
		for {
			select {
			case report := <-incoming:
				select {
				case out <- report:
				case <-sub.done:
					return
				}
			case <-sub.done:
				return
			}
		}
	}()

	// stop unsubscribes this one topic and releases the subscription. Disconnect/connection
	// loss release it in bulk via reportSub.close instead.
	stop := func() {
		client.unsubscribe(sub.topic)
		sub.close()
		client.removeSubscription(sub)
	}
	return out, stop, nil
}

// decodeEnvelope extracts the event name and raw data from a report message payload. The
// base Device.ParseEvent uses it; generated ParseEvent implementations reuse that via the
// embedded Device rather than calling this directly.
func decodeEnvelope(payload []byte) (event string, data json.RawMessage, ok bool) {
	var envelope struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", nil, false
	}
	return envelope.Event, envelope.Data, true
}

// deviceIdFromTopic extracts the device id from a report topic of the form
// yl-home/{homeId}/{deviceId}/report.
func deviceIdFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
