package main

import (
	"log"
	"net"

	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/packet"
)

func main() {
	intf, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatal(err)
	}
	ethSocket, err := packet.Listen(intf, packet.Raw, 0, nil)
	if err != nil {
		log.Printf("failed to packet.Listen: %v", err)
	}

	defer ethSocket.Close()

	rarp, err := arp.NewRARP(intf.HardwareAddr)
	if err != nil {
		log.Fatalf("failed to arp.NewPacket: %v", err)
	}
	log.Println(rarp)

	rarpBinary, err := rarp.MarshalBinary()
	if err != nil {
		log.Fatalf("error serializing RARP packet: %s", err)
	}

	const (
		EtherTypeRARP ethernet.EtherType = 0x8035
	)

	ethFrame := &ethernet.Frame{
		Destination: ethernet.Broadcast,
		Source:      intf.HardwareAddr,
		EtherType:   EtherTypeRARP,
		Payload:     rarpBinary,
	}

	addr := &packet.Addr{HardwareAddr: ethernet.Broadcast}

	b, err := ethFrame.MarshalBinary()
	if err != nil {
		log.Fatalf("Error in rarp.MarshalBinary: %s\n", err)
	}

	if _, err := ethSocket.WriteTo(b, addr); err != nil {
		log.Fatalf("emitFrame failed: %s", err)
	}

}
