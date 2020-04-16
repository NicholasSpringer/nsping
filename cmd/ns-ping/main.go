package main

import (
	"fmt"
	"time"

	"github.com/NicholasSpringer/nsping/nsping"
)

func main() {
	pinger, err := nsping.CreatePinger("google.com", 1*time.Second)
	if err != nil {
		fmt.Printf("Error creating pinger: %s\n", err.Error())
	}

	pinger.OnRecvPkt = func(info *nsping.PktInfo) {
		fmt.Printf("%d bytes from %s: icmp_seq=%d time=%.3f ms\n",
			info.NumBytes, pinger.Addr.String(), info.SeqNum, info.Rtt.Seconds()*1000.0)
	}

	pinger.OnFinish = func(s *nsping.PingStats) {
		percentLoss := 100.0 * float32(s.NumReceived) / float32(s.NumTransmitted)
		fmt.Printf("--- %s.com ping statistics ---", pinger.Host)
		fmt.Printf("%d packets transmitted, %d packets received, %.1f%% packet loss",
			s.NumTransmitted, s.NumReceived, percentLoss)
		fmt.Printf("round-trip min/avg/max = %.3f/%.3f/%.3f ms",
			s.Min.Seconds()*1000.0, s.Avg.Seconds()*1000.0, s.Max.Seconds()*1000.0)
	}

	pinger.Start()
}
