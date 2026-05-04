package motor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"
)

type Client struct {
	logger *slog.Logger
	addr   *net.UDPAddr
	conn   *net.UDPConn
	seq    atomic.Uint64
}

func NewClient(address string, logger *slog.Logger) (*Client, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolve udp address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}

	return &Client{
		logger: logger,
		addr:   addr,
		conn:   conn,
	}, nil
}

func (c *Client) Send(left, right int) error {
	msg := Message{
		Version:   ProtocolVersion,
		Type:      MessageTypeControl,
		Seq:       c.nextSeq(),
		Timestamp: nowUnixMilli(),
		Payload: Payload{
			Drive: &DrivePayload{
				Left:  left,
				Right: right,
			},
		},
	}

	return c.sendMessage(msg)
}

func (c *Client) Stop() error {
	return c.Send(0, 0)
}

func (c *Client) EmergencyStop(reason string) error {
	msg := Message{
		Version:   ProtocolVersion,
		Type:      MessageTypeSystem,
		Seq:       c.nextSeq(),
		Timestamp: nowUnixMilli(),
		Payload: Payload{
			System: &SystemPayload{
				Command: SystemCommandEmergencyStop,
				Reason:  reason,
			},
		},
	}

	return c.sendMessage(msg)
}

func (c *Client) sendMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal motor message: %w", err)
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		return fmt.Errorf("set udp write deadline: %w", err)
	}

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("write udp motor message: %w", err)
	}

	c.logger.Info(
		"motor message sent",
		"address", c.addr.String(),
		"type", msg.Type,
		"seq", msg.Seq,
		"payload", string(data),
	)

	return nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("close udp connection: %w", err)
	}

	return nil
}

func (c *Client) nextSeq() uint64 {
	return c.seq.Add(1)
}

func nowUnixMilli() int64 {
	return time.Now().UnixMilli()
}