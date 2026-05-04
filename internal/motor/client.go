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
	cmd := Command{
		Seq:   c.seq.Add(1),
		Left:  left,
		Right: right,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal motor command: %w", err)
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		return fmt.Errorf("set udp write deadline: %w", err)
	}

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("write udp motor command: %w", err)
	}

	c.logger.Info(
		"motor command sent",
		"address", c.addr.String(),
		"seq", cmd.Seq,
		"left", cmd.Left,
		"right", cmd.Right,
	)

	return nil
}

func (c *Client) Stop() error {
	return c.Send(0, 0)
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