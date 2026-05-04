package control

type DriveCommand struct {
	Left  int
	Right int
}

func StopCommand() DriveCommand {
	return DriveCommand{
		Left:  0,
		Right: 0,
	}
}