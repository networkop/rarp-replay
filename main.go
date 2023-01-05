package main

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

func inARP(rarp gopacket.Packet, intf *net.Interface, ip netip.Addr) ([]byte, error) {
	// gopacket does not understand RARP so using arp package instead
	var rarpFrame ethernet.Frame
	if err := rarpFrame.UnmarshalBinary(rarp.Data()); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal into ethernet packet: %s", err)
	}
	var rarpPayload arp.Packet
	if err := rarpPayload.UnmarshalBinary(rarpFrame.Payload); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal into arp packet: %s", err)
	}

	p, err := arp.NewPacket(
		arp.OperationRequest,           // Operation
		intf.HardwareAddr,              // srcHW
		ip,                             // srcIP
		rarpPayload.TargetHardwareAddr, // dstHW
		netip.MustParseAddr("0.0.0.0"), // dstIP
	)
	if err != nil {
		return nil, fmt.Errorf("failed to arp.NewPacket: %v", err)
	}
	log.Println(rarp)

	inarpBinary, err := p.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("error serializing RARP packet: %s", err)
	}

	ethFrame := &ethernet.Frame{
		Destination: ethernet.Broadcast,
		Source:      intf.HardwareAddr,
		EtherType:   ethernet.EtherTypeARP,
		Payload:     inarpBinary,
	}

	return ethFrame.MarshalBinary()
}

func arpProbe(intf *net.Interface) error {

	handle, err := pcap.OpenLive(
		intf.Name,         // interface name
		65536,             // snaplen (packet size)
		true,              // promisc
		pcap.BlockForever, // timeout
	)
	if err != nil {
		return err
	}
	defer handle.Close()

	handle.SetBPFFilter("rarp")

	packetSource := gopacket.NewPacketSource(
		handle,
		layers.LayerTypeEthernet,
	)

	addrs, err := intf.Addrs()
	if err != nil {
		return err
	}
	var addr netip.Addr
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				addr, _ = netip.AddrFromSlice(ip4)
				break
			}
		}
	}

	for packet := range packetSource.Packets() {
		log.Println("Found RARP packet")

		probe, err := inARP(packet, intf, addr)
		if err != nil {
			return err
		}
		if err := handle.WritePacketData(probe); err != nil {
			log.Fatalf("Failed to send packet data :%s\n", err)
		}
	}
	return nil
}

func main() {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, iface := range ifaces {
		wg.Add(1)
		go func(iface net.Interface) {
			defer wg.Done()
			if err := arpProbe(&iface); err != nil {
				log.Printf("interface %v: %v", iface.Name, err)
			}
		}(iface)
	}

	wg.Wait()
}
