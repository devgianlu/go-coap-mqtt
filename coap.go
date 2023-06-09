package main

import (
	"fmt"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/udp"
	udpClient "github.com/plgd-dev/go-coap/v3/udp/client"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type CoapServer struct {
	udpConn *net.UDPConn
	mux     *mux.Router
}

func NewCoapServer(serverPort int) (*CoapServer, error) {
	server := &CoapServer{mux: mux.NewRouter()}

	var err error
	server.udpConn, err = net.NewListenUDP("udp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		return nil, err
	}

	return server, nil
}

func (c *CoapServer) HandleResource(pattern string, handler func(w mux.ResponseWriter, r *mux.Message) (codes.Code, string)) {
	c.mux.HandleFunc(pattern, func(w mux.ResponseWriter, r *mux.Message) {
		// Handle the message
		code, data := handler(w, r)

		// Write response message
		if err := SendCoapMessage(w.Conn(), r.Token(), func(msg *pool.Message) {
			msg.SetCode(code)

			// If we have data, set it
			if len(data) > 0 {
				msg.SetBody(strings.NewReader(data))
				msg.SetContentFormat(message.TextPlain)
			}
		}); err != nil {
			log.WithError(err).Errorf("failed sending coap message to %v", w.Conn().RemoteAddr())
		}
	})
}

func (c *CoapServer) Serve() error {
	return udp.NewServer(
		options.WithMux(c.mux),
		options.WithKeepAlive(8, 2*time.Second, func(cc *udpClient.Conn) {
			log.Debugf("client at %s became inactive", cc.RemoteAddr())
		}),
	).Serve(c.udpConn)
}

func SendCoapMessage(cc mux.Conn, token []byte, f func(msg *pool.Message)) error {
	m := cc.AcquireMessage(cc.Context())
	defer cc.ReleaseMessage(m)
	m.SetToken(token)
	f(m)
	return cc.WriteMessage(m)
}
