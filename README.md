# go-coap-mqtt

`go-coap-mqtt` is a CoAP <-> MQTT gateway written in Go.

Messages can be sent and received from both MQTT and CoAP. The following ways of exchanging messages are supported:
- Publishing an MQTT message to a particular topic
- Making a CoAP `POST /pub` request with a JSON body containing the topic and payload
- Subscribing to a particular MQTT topic
- Observing the CoAP `GET /sub/{topic}` endpoint

## Testing

The `test` container has two CLI tools installed for interacting with the gateway and MQTT broker: `mqtt` and `coap-cli`.
You can get an interactive shell inside the container with `docker compose exec test bash`.

To make a CoAP request of the `/pub` endpoint use:

```shell
coap-cli post pub -h gateway -p 5688 -d '{"topic":"test","payload":"hello from coap"}' 
```

To make a CoAP request to the `/sub` endpoint use:

```shell
coap-cli get sub/test -h gateway -p 5688 -o
```

To publish an MQTT message use:

```shell
mqtt pub --host mqtt-broker --topic test -m "hello from mqtt"
```

To subscribe to MQTT messages use:

```shell
mqtt sub --host mqtt-broker --topic test
```