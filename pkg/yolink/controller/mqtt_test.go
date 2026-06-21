package controller

import "testing"

func TestMqttTopicFormatting(t *testing.T) {
	client := &MqttClient{homeId: "home1"}
	if got := client.reportTopic("d1"); got != "yl-home/home1/d1/report" {
		t.Errorf("reportTopic = %q", got)
	}
	if got := client.responseTopic("d1"); got != "yl-home/home1/d1/response" {
		t.Errorf("responseTopic = %q", got)
	}
	// The home-wide report subscription uses the "+" wildcard for the device segment.
	if got := client.reportTopic("+"); got != "yl-home/home1/+/report" {
		t.Errorf("wildcard reportTopic = %q", got)
	}
}

func TestSubscribeRequiresConnection(t *testing.T) {
	client := newMqttClient() // paho is nil until connect
	if err := client.Subscribe("topic", func(MqttMessage) {}); err == nil {
		t.Fatal("expected error subscribing before connect")
	}
}

func TestDecodeEnvelope(t *testing.T) {
	event, data, ok := decodeEnvelope([]byte(`{"event":"DoorSensor.Report","data":{"state":"open"}}`))
	if !ok || event != "DoorSensor.Report" || string(data) != `{"state":"open"}` {
		t.Fatalf("decodeEnvelope = %q, %s, %v", event, data, ok)
	}
	if _, _, ok := decodeEnvelope([]byte("not json")); ok {
		t.Fatal("expected ok=false for bad payload")
	}
}
