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
	"github.com/vishvananda/netlink"
	"golang.org/x/net/bpf"
)

type probe struct {
	intf *net.Interface
	addr netip.Addr
}

func buildARP(rarp gopacket.Packet, intf *net.Interface, ip netip.Addr) (*ethernet.Frame, error) {
	// gopacket does not understand RARP so using mdlayher/ethernet and mdlayher/arp packages instead
	var rarpFrame ethernet.Frame
	if err := rarpFrame.UnmarshalBinary(rarp.Data()); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal into ethernet packet: %s", err)
	}
	var rarpPayload arp.Packet
	if err := rarpPayload.UnmarshalBinary(rarpFrame.Payload); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal into arp packet: %s", err)
	}

	neighs, err := netlink.NeighList(intf.Index, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("Failed to get a list of ARP/ND neighbors: %s", err)
	}

	var targetIP netip.Addr
	targetHWAddr := rarpPayload.TargetHardwareAddr.String()
	for _, neigh := range neighs {
		if neigh.HardwareAddr.String() != targetHWAddr {
			continue
		}
		targetIP, _ = netip.AddrFromSlice(neigh.IP)
		break
	}
	if !targetIP.IsValid() {
		return nil, fmt.Errorf("Could not find a neighbor matching %s", targetHWAddr)
	}

	p, err := arp.NewPacket(
		arp.OperationRequest,           // Operation
		intf.HardwareAddr,              // srcHW
		ip,                             // srcIP
		rarpPayload.TargetHardwareAddr, // dstHW
		targetIP,                       // dstIP
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

func (p *probe) start() error {
	log.Printf("Starting a probe on %s@%s", p.addr, p.intf.Name)

	// using pcap to catch the incoming RARP packets
	handle, err := pcapgo.NewEthernetHandle(p.intf.Name)
	if err != nil {
		return err
	}
	defer handle.Close()
	log.Printf("Capturing packets on %s", p.intf.Name)

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

	// setting up a raw socket for sending the ARP packets
	ethSocket, err := packet.Listen(p.intf, packet.Raw, 0, nil)
	if err != nil {
		log.Fatalf("failed to set up a raw socket: %v", err)
	}
	defer ethSocket.Close()
	log.Printf("Opened a raw socket %s", p.intf.Name)

	for pkt := range packetSource.Packets() {
		log.Println("Received RARP packet on %s", p.intf.Name)

		probe, err := buildARP(pkt, p.intf, p.addr)
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
		log.Println("Sent an ARP packet on %s to %s", p.intf.Name, probe.Destination.String())

	}
	return nil
}

func newProbe(intf net.Interface) (*probe, error) {
	p := &probe{intf: &intf}

	// find the first usable IP address
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
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
	if !addr.IsValid() || addr.IsLoopback() {
		return nil, fmt.Errorf("Could not find a valid address on %s", intf.Name)
	}
	p.addr = addr

	return p, nil
}

func main() {
	intfs, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, intf := range intfs {
		p, err := newProbe(intf)
		if err != nil {
			log.Print(err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.start(); err != nil {
				log.Printf("interface %v: %v", intf.Name, err)
			}
		}()

	}

	wg.Wait()
}
