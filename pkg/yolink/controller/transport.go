package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tyandl/homelink/pkg/yolink/types"
)

// connection is the shared transport to the YoLink API: an HTTP client always, plus an MQTT
// client once InitializeMqtt is called. One connection is shared by the Home and every device
// (each wrapped in its own clientContext).
type connection struct {
	httpClient *HttpClient

	// mqttMu guards mqttClient, which InitializeMqtt/CloseMqtt create and clear and the
	// async/subscribe paths read.
	mqttMu     sync.Mutex
	mqttClient *MqttClient
}

// mqtt returns the current MQTT client (nil if not initialized) under mqttMu.
func (c *connection) mqtt() *MqttClient {
	c.mqttMu.Lock()
	defer c.mqttMu.Unlock()
	return c.mqttClient
}

// AsyncResult is the outcome of an asynchronous device call: the decoded typed response,
// or an error (timeout, transport failure, or decode failure).
type AsyncResult[T any] struct {
	Response T
	Err      error
}

// RawResponse is the undecoded outcome of an asynchronous call: the raw payload bytes, or an
// error. DecodeAsync turns it into a typed AsyncResult[T], which keeps this transport path
// itself non-generic.
type RawResponse struct {
	data json.RawMessage
	err  error
}

// newConnection establishes the shared HTTP connection to the YoLink API.
func newConnection(httpHost string, clientId func() string, clientSecret func() string) (*connection, error) {
	httpClient, err := connectHttp(httpHost, clientId, clientSecret)
	if err != nil {
		return nil, err
	}
	return &connection{httpClient: httpClient, mqttClient: nil}, nil
}

// InitializeMqtt builds and connects the MQTT client, authenticating with the access token.
// homeId scopes the report/request/response topics. No-op if already connected.
func (c *connection) InitializeMqtt(mqttBroker string, homeId string) error {
	c.mqttMu.Lock()
	defer c.mqttMu.Unlock()
	if c.mqttClient != nil {
		return nil // already initialized
	}
	accessToken, err := c.httpClient.AccessToken()
	if err != nil {
		return err
	}
	client := newMqttClient()
	if err := client.connect(mqttBroker, accessToken, homeId, func(err error) {
		slog.Error("mqtt connection lost", "error", err)
	}); err != nil {
		return err
	}
	c.mqttClient = client
	slog.Info("mqtt initialized", "homeId", homeId)
	return nil
}

// CloseMqtt disconnects the MQTT client (releasing its subscriptions and failing any
// in-flight calls), then clears it from the shared connection so a later InitializeMqtt can
// reconnect.
func (c *connection) CloseMqtt() {
	c.mqttMu.Lock()
	client := c.mqttClient
	c.mqttClient = nil
	c.mqttMu.Unlock()
	if client != nil {
		client.Disconnect() // outside the lock: it fails in-flight calls and closes channels
		slog.Info("mqtt closed")
	}
}

// deviceOf returns the *Device a controller is backed by, or nil for non-device controllers
// (e.g. Home).
func deviceOf(controller DeviceController) *Device {
	dev := controller
	if dc, ok := dev.(deviceContainer); ok {
		dev = dc.getDevice()
	}
	if d, ok := dev.(*Device); ok {
		return d
	}
	return nil
}

// call synchronously invokes method against device (nil for home-level calls), adding its
// deviceId and token, and waits for the reply. When response is non-nil, the reply payload
// is unmarshaled into it. For a device call with MQTT connected, it routes over MQTT
// (awaiting the async result) instead of HTTP.
func (c *connection) call(controller DeviceController, method string, params any, response any) error {
	if c == nil || c.httpClient == nil {
		return fmt.Errorf("No associated HTTP client for making API calls")
	}
	device := deviceOf(controller)

	// Prefer MQTT for device commands when a connection is available; await the result so
	// this call stays synchronous.
	if device != nil && c.mqtt() != nil {
		slog.Debug("routing device call over mqtt", "method", method, "device", device.deviceId)
		result := <-c.callAsync(controller, method, params)
		if result.err != nil {
			return result.err
		}
		if response != nil {
			return json.Unmarshal(result.data, response)
		}
		return nil
	}

	var targetDeviceId, token *string
	if device != nil {
		targetDeviceId, token = &device.deviceId, &device.token
	}
	bddp := newBDDP(method, params, targetDeviceId, token)
	bddp.MsgId = types.NewMessageId()
	var budp basicUploadDataPacket
	if err := c.httpClient.Post(bddp, &budp); err != nil {
		return err
	}
	if response != nil {
		return json.Unmarshal(budp.Data, response)
	}
	return nil
}

// callAsync publishes an MQTT command for device and returns a channel with the raw
// response. It is non-generic; typed decoding is left to the caller (the generated device
// *Async methods).
func (c *connection) callAsync(controller DeviceController, method string, params any) <-chan RawResponse {
	out := make(chan RawResponse, 1)
	if c == nil {
		out <- RawResponse{err: fmt.Errorf("No associated MQTT client for making API calls")}
		return out
	}
	mqtt := c.mqtt()
	if mqtt == nil {
		out <- RawResponse{err: fmt.Errorf("No associated MQTT client for making API calls")}
		return out
	}
	device := deviceOf(controller)
	if device == nil {
		out <- RawResponse{err: fmt.Errorf("CallAsync requires a target device")}
		return out
	}
	bddp := newBDDP(method, params, &device.deviceId, &device.token)
	bddp.MsgId = types.NewMessageId()
	go func() {
		result := <-mqtt.call(device.deviceId, bddp)
		out <- RawResponse{data: result.Response.Data, err: result.Err}
	}()
	return out
}

// subscribe subscribes to report events for controller: one device's reports, or every
// device's (the "+" wildcard) when controller is not a device (e.g. Home). It returns a
// channel of parsed reports and a stop function that unsubscribes and ends delivery (closing
// the channel) once the receiver is done listening.
func (c *connection) subscribe(controller DeviceController) (<-chan Report, func(), error) {
	mqtt := c.mqtt()
	if mqtt == nil {
		return nil, nil, fmt.Errorf("no associated MQTT client for subscribing")
	}
	deviceTopic := "+" // home-wide default: match every device's report topic
	if device := deviceOf(controller); device != nil {
		deviceTopic = device.deviceId
	}
	return subscribeReports(mqtt, mqtt.reportTopic(deviceTopic), controller)
}

// DecodeAsync adapts a raw response channel into a typed result channel, unmarshaling the
// payload into T once it arrives. It lets the generated typed *Async methods stay one-liners
// over the non-generic connection.callAsync.
func DecodeAsync[T any](raw <-chan RawResponse) <-chan AsyncResult[T] {
	out := make(chan AsyncResult[T], 1)
	go func() {
		result := <-raw
		if result.err != nil {
			out <- AsyncResult[T]{Err: result.err}
			return
		}
		var response T
		if len(result.data) > 0 {
			if err := json.Unmarshal(result.data, &response); err != nil {
				out <- AsyncResult[T]{Err: err}
				return
			}
		}
		out <- AsyncResult[T]{Response: response}
	}()
	return out
}
