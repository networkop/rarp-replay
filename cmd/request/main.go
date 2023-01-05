package main

import (
	"flag"
	"log"
	"net"

	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/packet"
)

const EtherTypeRARP ethernet.EtherType = 0x8035

var (
	macFlag  = flag.String("mac", "", "MAC address for RARP request")
	intfFlag = flag.String("intf", "", "interface for RARP request")
)

func main() {
	flag.Parse()
	if len(*macFlag) < 1 {
		log.Fatal("MAC address must be provided")
	}
	if len(*intfFlag) < 1 {
		log.Fatal("interface must be provided")
	}

	rarpMAC, err := net.ParseMAC(*macFlag)
	if err != nil {
		log.Fatalf("Invalid MAC address provided: %v", err)
	}

	intf, err := net.InterfaceByName(*intfFlag)
	if err != nil {
		log.Fatal(err)
	}
	ethSocket, err := packet.Listen(intf, packet.Raw, 0, nil)
	if err != nil {
		log.Fatalf("failed to packet.Listen: %v", err)
	}

	defer ethSocket.Close()

	rarp, err := arp.NewRARP(rarpMAC)
	if err != nil {
		log.Fatalf("failed to arp.NewPacket: %v", err)
	}
	log.Println(rarp)

	rarpBinary, err := rarp.MarshalBinary()
	if err != nil {
		log.Fatalf("error serializing RARP packet: %s", err)
	}

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
