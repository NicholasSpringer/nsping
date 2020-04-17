package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NicholasSpringer/nsping/nsping"
)

const usage = `usage: sudo nsping [-i interval (ms)] [-m ttl] host`

func main() {
	var interval = flag.Int("i", 1000, "interval in ms (minimum 100)")
	var ttl = flag.Int("m", 64, "Time to live for outgoing packets")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Println(usage)
		return
	}
	if *interval < 100 {
		fmt.Println("Error: minimum interval is 500 ms")
	}

	host := flag.Arg(0)
	i := time.Duration(*interval) * time.Millisecond
	pinger, err := nsping.CreatePinger(host, i, *ttl)
	if err != nil {
		fmt.Printf("Error creating pinger: %s\n", err.Error())
		return
	}
	pinger.OnRecvPkt = func(info *nsping.PktInfo) {
		estimatedLoss :=
			100.0 * float32(info.EstimatedLost) / (float32(info.NumReceived) + float32(info.EstimatedLost))
		fmt.Printf("response from %s: icmp_seq=%d ttl=%d time=%.3f ms estimated loss: %.1f%%\n",
			pinger.Addr.String(), info.SeqNum, info.TTL, info.RTT.Seconds()*1000.0, estimatedLoss)
	}
	pinger.OnAssumeLost = func(seq int) {
		fmt.Printf("icmp_seq %d assumed to be lost\n", seq)
	}
	pinger.OnFinish = func(s *nsping.PingStats) {
		percentLoss := 100.0 * float32(s.NumTransmitted-s.NumReceived) / float32(s.NumTransmitted)
		fmt.Println()
		fmt.Printf("--- %s ping statistics ---\n", pinger.Host)
		fmt.Printf("%d packets transmitted, %d packets received, %.1f%% packet loss\n",
			s.NumTransmitted, s.NumReceived, percentLoss)
		fmt.Printf("round-trip min/avg/max = %.3f/%.3f/%.3f ms\n",
			s.Min.Seconds()*1000.0, s.Avg.Seconds()*1000.0, s.Max.Seconds()*1000.0)
	}
	go pinger.Run()

	sigChan := make(chan os.Signal, 3)
	// Intercept signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	// Wait for signal
	<-sigChan
	pinger.FinishChan <- 1
	// Wait for channel to close when pinger finishes
	<-pinger.FinishChan
}
