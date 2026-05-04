package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"time"
)

type MotorTelemetryMessage struct {
	Type           string  `json:"type"`
	BatteryVoltage float64 `json:"battery_voltage"`
	RSSI           int     `json:"rssi"`
	Left           int     `json:"left"`
	Right          int     `json:"right"`
	Failsafe       bool    `json:"failsafe"`
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

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	log.Println("telemetry sender started")

	for range ticker.C {
		msg := MotorTelemetryMessage{
			Type:           "motor_status",
			BatteryVoltage: 7.2 + rand.Float64()*0.3,
			RSSI:           -45 - rand.Intn(20),
			Left:           0,
			Right:          0,
			Failsafe:       false,
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