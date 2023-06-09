package main

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"time"
)

type MqttClient struct {
	client mqtt.Client
}

func NewMqttClient(broker, clientID string) (*MqttClient, error) {
	client := &MqttClient{}
	client.client = mqtt.NewClient(
		mqtt.NewClientOptions().
			AddBroker(broker).
			SetClientID(clientID).
			SetKeepAlive(2 * time.Second).
			SetPingTimeout(1 * time.Second),
	)

	if token := client.client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed connecting to mqtt broker: %w", token.Error())
	}

	return client, nil
}

func (c *MqttClient) Subscribe(topic string, callback mqtt.MessageHandler) error {
	if token := c.client.Subscribe(topic, 0, callback); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (c *MqttClient) Publish(topic string, payload string) error {
	if token := c.client.Publish(topic, 0, false, []byte(payload)); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}
