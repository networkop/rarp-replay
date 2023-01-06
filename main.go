package main

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/packet"
	"golang.org/x/net/bpf"
)

// constructor for the (inverse) ARP packet
func inARP(rarp gopacket.Packet, intf *net.Interface, ip netip.Addr) (*ethernet.Frame, error) {
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
	log.Println(p)

	inarpBinary, err := p.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("error serializing RARP packet: %s", err)
	}

	return &ethernet.Frame{
		Destination: rarpPayload.TargetHardwareAddr,
		Source:      intf.HardwareAddr,
		EtherType:   ethernet.EtherTypeARP,
		Payload:     inarpBinary,
	}, nil
}

func arpProbe(intf *net.Interface) error {

	// using pcap to catch the incoming RARP packets
	handle, err := pcapgo.NewEthernetHandle(intf.Name)
	if err != nil {
		return err
	}
	defer handle.Close()

	rawInstructions, err := bpf.Assemble([]bpf.Instruction{
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x8035, SkipTrue: 1},
		bpf.RetConstant{Val: 4096},
		bpf.RetConstant{Val: 0},
	})

	handle.SetBPF(rawInstructions)

	packetSource := gopacket.NewPacketSource(
		handle,
		layers.LayerTypeEthernet,
	)

	// find the first L3 address on the interface
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

	// setting up a raw socket for sending the ARP packets
	ethSocket, err := packet.Listen(intf, packet.Raw, 0, nil)
	if err != nil {
		log.Fatalf("failed to set up a raw socket: %v", err)
	}
	defer ethSocket.Close()

	for p := range packetSource.Packets() {
		log.Println("Found RARP packet")

		probe, err := inARP(p, intf, addr)
		if err != nil {
			return err
		}

		data, err := probe.MarshalBinary()
		if err != nil {
			return fmt.Errorf("Failed to marshal ARP frame: %s", err)
		}

		dstAddr := &packet.Addr{HardwareAddr: probe.Destination}

		if _, err := ethSocket.WriteTo(data, dstAddr); err != nil {
			log.Fatalf("ethSocket.WriteTo failed: %s", err)
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
