package nsping

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	icmpProtoNum   = 1
	icmpHeaderSize = 8
	payloadSize    = 15
)

type Pinger struct {
	Host string
	Addr net.Addr
	conn *ipv4.PacketConn

	OnRecvPkt    func(*PktInfo)
	OnAssumeLost func(int)
	OnFinish     func(*PingStats)
	FinishChan   chan int

	interval time.Duration

	maxSeqRecv    int
	estimatedLost int

	statistics PingStats
}

type PingStats struct {
	NumTransmitted int
	NumReceived    int
	Min            time.Duration
	Max            time.Duration
	Avg            time.Duration
}

type PktInfo struct {
	SeqNum        int
	RTT           time.Duration
	NumBytes      int
	TTL           int
	NumReceived   int
	EstimatedLost int
}

type recvPkt struct {
	bytes    []byte
	cm       *ipv4.ControlMessage
	from     net.Addr
	timeRecv time.Time
}

func CreatePinger(host string, interval time.Duration, ttl int) (p *Pinger, err error) {
	finishChan := make(chan int)

	addr, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		return
	}
	packetConn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return
	}
	conn := packetConn.IPv4PacketConn()
	if err = conn.SetControlMessage(1, true); err != nil {
		return
	}
	if err = conn.SetTTL(ttl); err != nil {
		return
	}

	p = &Pinger{
		Host:       host,
		Addr:       addr,
		conn:       conn,
		FinishChan: finishChan,
		interval:   interval,
		maxSeqRecv: -1,
		statistics: PingStats{},
	}

	return
}

func (p *Pinger) Run() {
	sendTicker := time.NewTicker(p.interval)
	recvChan := make(chan *recvPkt)

	go p.recvPkts(recvChan)

	for {
		select {
		case <-p.FinishChan:
			sendTicker.Stop()
			err := p.conn.SetReadDeadline(time.Now())
			if err != nil {
				return
			}
			p.OnFinish(&p.statistics)
			close(p.FinishChan)
			return
		case received := <-recvChan:
			info, isInvalid, err := p.processPkt(received)
			if err != nil {
				fmt.Printf("nsping: error processing packet: %s\n", err.Error())
				continue
			}
			if isInvalid {
				fmt.Printf("invalid")
				continue
			}
			p.statistics.updateStats(info)
			info.NumReceived = p.statistics.NumReceived

			if info.SeqNum > p.maxSeqRecv {
				// Number of packets missing between last received and this packet
				numSkipped := info.SeqNum - (p.maxSeqRecv + 1)
				p.estimatedLost += numSkipped
				for i := p.maxSeqRecv + 1; i < info.SeqNum; i++ {
					p.OnAssumeLost(i)
				}
			} else {
				// Packet was previously assumed to be lost, decrement num lost
				p.estimatedLost--
			}
			p.maxSeqRecv = info.SeqNum
			info.EstimatedLost = p.estimatedLost
			p.OnRecvPkt(info)
		case <-sendTicker.C:
			err := p.sendPkt()
			if err != nil {
				fmt.Printf("nsping: error sending packet: %s\n", err.Error())
				continue
			}
			p.statistics.NumTransmitted++
		}
	}
}

func (p *Pinger) sendPkt() (err error) {
	payload, err := time.Now().MarshalBinary()
	if err != nil {
		return
	}
	msgBody := icmp.Echo{
		ID:   0,
		Seq:  p.statistics.NumTransmitted,
		Data: payload,
	}

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &msgBody,
	}

	msgBytes, err := msg.Marshal(nil)
	_, err = p.conn.WriteTo(msgBytes, &ipv4.ControlMessage{}, p.Addr)
	return
}

func (p *Pinger) recvPkts(recvChan chan<- *recvPkt) {
	for {
		buf := make([]byte, icmpHeaderSize+payloadSize)
		_, cm, from, err := p.conn.ReadFrom(buf)
		if err != nil {
			// Finish if ReadFrom timed out
			if err, ok := err.(net.Error); ok && err.Timeout() {
				close(recvChan)
				return
			}
			fmt.Printf("nsping: error receiving packet: %s\n", err.Error())
			continue
		}

		recvChan <- &recvPkt{bytes: buf, cm: cm, from: from, timeRecv: time.Now()}
	}
}

func (p *Pinger) processPkt(received *recvPkt) (info *PktInfo, isInvalid bool, err error) {
	msg, err := icmp.ParseMessage(icmpProtoNum, received.bytes)
	if err != nil {
		return
	}
	body, ok := msg.Body.(*icmp.Echo)
	if !ok || msg.Type != ipv4.ICMPTypeEchoReply || received.from.String() != p.Addr.String() {
		isInvalid = true
		return
	}

	var timeSent time.Time
	err = timeSent.UnmarshalBinary(body.Data)
	if err != nil {
		return
	}

	rtt := received.timeRecv.Sub(timeSent)
	ttl := 0
	if received.cm != nil {
		ttl = received.cm.TTL
	}

	info = &PktInfo{
		SeqNum:   body.Seq,
		RTT:      rtt,
		TTL:      ttl,
		NumBytes: len(received.bytes),
	}
	return
}

func (s *PingStats) updateStats(info *PktInfo) {
	s.NumReceived++
	if s.Min == time.Duration(0) || info.RTT < s.Min {
		s.Min = info.RTT
	}
	if s.Max == time.Duration(0) || info.RTT > s.Max {
		s.Max = info.RTT
	}
	avgFloat := (float64(s.NumReceived-1)/float64(s.NumReceived))*
		float64(s.Avg) +
		(1/float64(s.NumReceived))*float64(info.RTT)
	s.Avg = time.Duration(avgFloat)
}
