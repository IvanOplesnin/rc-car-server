package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"sync/atomic"
	"time"
)

const (
	protocolVersion      = 1
	messageTypeTelemetry = "telemetry"
)

type Message struct {
	Version   int     `json:"version"`
	Type      string  `json:"type"`
	Seq       uint64  `json:"seq"`
	Timestamp int64   `json:"timestamp"`
	Payload   Payload `json:"payload"`
}

type Payload struct {
	Motor   MotorPayload   `json:"motor"`
	Power   PowerPayload   `json:"power"`
	Network NetworkPayload `json:"network"`
	System  SystemPayload  `json:"system"`
}

type MotorPayload struct {
	Left     int  `json:"left"`
	Right    int  `json:"right"`
	Failsafe bool `json:"failsafe"`
}

type PowerPayload struct {
	BatteryVoltage float64 `json:"battery_voltage"`
	BatteryPercent int     `json:"battery_percent"`
}

type NetworkPayload struct {
	RSSI int `json:"rssi"`
}

type SystemPayload struct {
	UptimeMS uint64 `json:"uptime_ms"`
	FreeHeap uint64 `json:"free_heap"`
}

func main() {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4211")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	var seq atomic.Uint64
	startedAt := time.Now()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	log.Println("telemetry sender started")

	for range ticker.C {
		uptime := uint64(time.Since(startedAt).Milliseconds())

		msg := Message{
			Version:   protocolVersion,
			Type:      messageTypeTelemetry,
			Seq:       seq.Add(1),
			Timestamp: time.Now().UnixMilli(),
			Payload: Payload{
				Motor: MotorPayload{
					Left:     0,
					Right:    0,
					Failsafe: false,
				},
				Power: PowerPayload{
					BatteryVoltage: 7.2 + rand.Float64()*0.3,
					BatteryPercent: 75 + rand.Intn(10),
				},
				Network: NetworkPayload{
					RSSI: -45 - rand.Intn(20),
				},
				System: SystemPayload{
					UptimeMS: uptime,
					FreeHeap: 100000 + uint64(rand.Intn(10000)),
				},
			},
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Println("marshal:", err)
			continue
		}

		if _, err := conn.Write(data); err != nil {
			log.Println("write:", err)
			continue
		}

		log.Println(string(data))
	}
}