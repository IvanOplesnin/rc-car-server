package control

import "errors"

var (
	ErrInvalidLeftSpeed  = errors.New("left speed must be between -100 and 100")
	ErrInvalidRightSpeed = errors.New("right speed must be between -100 and 100")
)