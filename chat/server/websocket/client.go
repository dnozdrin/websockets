package websocket

import (
	"bytes"
	"errors"
	ws "github.com/gorilla/websocket"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type Client struct {
	hub  *Hub
	log  Logger
	conn *ws.Conn
	send chan []byte
}

func startClient(hub *Hub, conn *ws.Conn, log Logger, bufferSize int) {
	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, bufferSize),
		log:  log,
	}

	hub.register <- client

	go client.write()
	go client.read()
}

func (c *Client) read() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.log.Error(err)
	}
	c.conn.SetPongHandler(func(string) error { _ = c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				c.log.Error(err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.hub.broadcast <- message
	}
}

func (c *Client) write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				if err := c.conn.WriteMessage(ws.CloseMessage, []byte{}); err != nil && !errors.Is(err, ws.ErrCloseSent) {
					c.log.Error(err)
				}
				return
			}

			if err := c.writeMessage(message); err != nil {
				c.log.Error(err)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(ws.PingMessage, nil); err != nil {
				c.log.Error(err)
				return
			}
		}
	}
}

func (c *Client) writeMessage(msg []byte) error {
	w, err := c.conn.NextWriter(ws.TextMessage)
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}

	n := len(c.send)
	for i := 0; i < n; i++ {
		if _, err := w.Write(newline); err != nil {
			return err
		}
		if _, err := w.Write(<-c.send); err != nil {
			return err
		}
	}

	if err := w.Close(); err != nil {
		return err
	}

	return nil
}
