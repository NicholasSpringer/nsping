package nsping
import (
	"golang.org/x/net/icmp"
)

type Pinger struct {
	addr string
	conn PacketConn
	output chan PacketInfo

	numLost int
}

type PacketInfo struct {
	Addr string
	SeqNum int
	rtt time.Duration
	NumBytes int
	Ttl int
	CumulLost int
}

func CreatePinger(addr string, ) output chan PacketInfo {

}

func (p *Pinger) sendPacket() {


	p.conn.write
}