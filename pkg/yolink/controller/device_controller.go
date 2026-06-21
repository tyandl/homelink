package controller

// DeviceController is implemented by every typed device in the devices package, by the
// untyped Device fallback, and by Home (which acts as the home-level controller).
type DeviceController interface {
	GetId() string
	GetName() string
	GetType() string
	GetModelName() string
	mqttSubscriber
	transporter
}

type transporter interface {
	Call(method string, params any, response any) error
	CallAsync(method string, params any) <-chan RawResponse
}

// deviceContainer is satisfied by any DeviceController backed by a Device — every concrete
// device embeds *Device, which provides the promoted getDevice. Home does not embed *Device,
// so a type assertion to deviceContainer tells real devices apart from Home (and yields the
// embedded *Device) without reflection.
type deviceContainer interface {
	getDevice() *Device
}

type mqttSubscriber interface {
	// ParseEvent decodes an MQTT report message into a Report. Home resolves the reporting
	// device from the message topic and delegates to it; a device parses with its own type.
	// Returns false if the payload can't be decoded.
	ParseEvent(message MqttMessage) (Report, bool)
	// Subscribe subscribes to this controller's report events, returning a channel of
	// parsed reports and a stop function that unsubscribes and ends delivery. Device
	// scopes this to its own reports; Home covers every device in the home.
	Subscribe() (<-chan Report, func(), error)
}

// clientContext pairs a shared connection with the controller it acts on behalf of: a
// Home for home-level calls, or a device's controller for device-addressed ones.
type clientContext struct {
	*connection
	controller DeviceController
}

// newClientContext binds an existing connection to a controller. The connection is
// created separately (newConnection) so this is a pure, non-failing binding — which also
// lets a Home bind itself after it exists.
func newClientContext(client *connection, controller DeviceController) *clientContext {
	return &clientContext{connection: client, controller: controller}
}

func (c *clientContext) Call(method string, params any, response any) error {
	return c.connection.call(c.controller, method, params, response)
}

func (c *clientContext) CallAsync(method string, params any) <-chan RawResponse {
	return c.connection.callAsync(c.controller, method, params)
}

func (c *clientContext) Subscribe() (<-chan Report, func(), error) {
	return c.connection.subscribe(c.controller)
}
