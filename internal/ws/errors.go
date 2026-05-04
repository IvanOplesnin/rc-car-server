package ws

import "errors"

var (
	ErrUnknownMessageType        = errors.New("unknown message type")
	ErrUnsupportedProtocolVersion = errors.New("unsupported websocket protocol version")
	ErrMissingDrivePayload       = errors.New("missing drive payload")
	ErrMissingSystemPayload      = errors.New("missing system payload")
	ErrUnknownSystemCommand      = errors.New("unknown system command")
)