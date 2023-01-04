package main

import (
	"log"
	"net"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func rarp(intf *net.Interface) error {

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
	for packet := range packetSource.Packets() {
		log.Println("Found RARP packet")

		if err := handle.WritePacketData(packet.Data()); err != nil {
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
			if err := rarp(&iface); err != nil {
				log.Printf("interface %v: %v", iface.Name, err)
			}
		}(iface)
	}

	wg.Wait()
}
