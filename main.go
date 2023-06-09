package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/mux"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
)

type Message struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
}

type MqttCoapSignal struct {
	Register bool
	Topic    string
	Conn     mux.Conn
	Token    []byte
}

func mqttToCoapLoop(messages chan *Message, signaling chan *MqttCoapSignal) {
	type Client struct {
		Conn  mux.Conn
		Topic string
		Token []byte
		Obs   uint32
	}

	clients := map[string]*Client{}

	// This loop handles messages and control signals
	for {
		select {
		case msg := <-messages:
			for _, c := range clients {
				if msg.Topic != c.Topic {
					continue
				}

				// Send the message to the given client
				if err := SendCoapMessage(c.Conn, c.Token, func(m *pool.Message) {
					m.SetCode(codes.Content)

					var body bytes.Buffer
					if err := json.NewEncoder(&body).Encode(msg); err != nil {
						log.WithError(err).Fatal("failed marshalling message")
					}

					m.SetObserve(c.Obs)
					c.Obs++

					m.SetBody(bytes.NewReader(body.Bytes()))
					m.SetContentFormat(message.AppJSON)
				}); err != nil {
					log.WithError(err).Errorf("failed sending coap message to %v", c.Conn.RemoteAddr())
				}
			}
		case signal := <-signaling:
			if signal.Register {
				clients[string(signal.Token)] = &Client{Conn: signal.Conn, Token: signal.Token, Topic: signal.Topic, Obs: 2}
			} else {
				delete(clients, string(signal.Token))
			}
		}
	}
}

func parseArgs() (string, string, int) {
	coapServerPort, err := strconv.ParseInt(os.Args[3], 10, 0)
	if err != nil {
		log.Fatalf("invalid coap server port: %s", err)
	}

	return os.Args[1], os.Args[2], int(coapServerPort)
}

func main() {
	mqttToCoapMessages := make(chan *Message)
	mqttToCoapSignaling := make(chan *MqttCoapSignal)

	if len(os.Args) != 4 {
		log.Infof("Usage: %s [mqtt broker] [mqtt client id] [coap server port]", os.Args[0])
		return
	}

	// Parse the three required arguments
	mqttBrokerAddr, mqttClientId, coapServerPort := parseArgs()

	// Initialize MQTT client
	mqttClient, err := NewMqttClient(mqttBrokerAddr, mqttClientId)
	if err != nil {
		log.WithError(err).Fatal("failed initializing mqtt client")
	}

	// Subscribe to MQTT messages to rely them on CoAP
	if err := mqttClient.Subscribe("#", func(c mqtt.Client, m mqtt.Message) {
		log.Debugf("[mqtt] got message %d", m.MessageID())

		mqttToCoapMessages <- &Message{Topic: m.Topic(), Payload: string(m.Payload())}
	}); err != nil {
		log.WithError(err).Fatal("failed subscribing to mqtt topic")
	}

	// Initialize CoAP server
	coapServer, err := NewCoapServer(coapServerPort)
	if err != nil {
		log.WithError(err).Fatal("failed initializing coap server")
	}

	// Publish a message from CoAP to MQTT
	coapServer.HandleResource("/pub", func(w mux.ResponseWriter, r *mux.Message) (codes.Code, string) {
		log.Debugf("[coap] got pub message %+v from %v", r, w.Conn().RemoteAddr())

		var msg Message
		if json.NewDecoder(r.Body()).Decode(&msg) != nil {
			log.WithError(err).Error("failed decoding coap pub message")

			// Let the CoAP client know that its message was invalid
			return codes.BadRequest, fmt.Sprintf("invalid JSON coap message: %s", err)
		}

		// Publish the message on MQTT
		if err := mqttClient.Publish(msg.Topic, msg.Payload); err != nil {
			log.WithError(err).Error("failed publishing mqtt message")

			// Let the CoAP client know that we failed to publish the message
			return codes.BadRequest, fmt.Sprintf("failed publishing message: %s", err)
		}

		// Let the CoAP client know that we published the message
		return codes.Valid, ""
	})

	// Subscribe to messages from MQTT with CoAP
	coapServer.HandleResource("/sub/{topic}", func(w mux.ResponseWriter, r *mux.Message) (codes.Code, string) {
		log.Debugf("[coap] got sub message %+v from %v", r, w.Conn().RemoteAddr())

		topic, ok := r.RouteParams.Vars["topic"]
		if !ok {
			return codes.BadRequest, "missing topic"
		}

		obs, err := r.Options().Observe()
		switch {
		case r.Code() == codes.GET && err == nil && obs == 0:
			// Let the CoAP client know that it was registered
			mqttToCoapSignaling <- &MqttCoapSignal{Register: true, Conn: w.Conn(), Token: r.Token(), Topic: topic}
			return codes.Content, ""
		case r.Code() == codes.GET && err == nil && obs == 1:
			// Let the CoAP client know that it was deregistered
			mqttToCoapSignaling <- &MqttCoapSignal{Register: false, Conn: w.Conn(), Token: r.Token(), Topic: topic}
			return codes.Content, ""
		default:
			// Let the CoAP client that the request was invalid
			return codes.BadRequest, "invalid observe request"
		}
	})

	// Start the loop for dispatching MQTT messages to CoAP observers
	go mqttToCoapLoop(mqttToCoapMessages, mqttToCoapSignaling)

	// Start the CoAP server
	if err := coapServer.Serve(); err != nil {
		log.WithError(err).Fatal("failed running coap server")
	}
}
