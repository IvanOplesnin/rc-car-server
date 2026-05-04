package motor

type Command struct {
	Seq   uint64 `json:"seq"`
	Left  int    `json:"left"`
	Right int    `json:"right"`
}