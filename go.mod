module github.com/networkop/rarp-replay

go 1.18

require (
	github.com/google/gopacket v1.1.19
	github.com/mdlayher/arp v0.0.0-20220512170110-6706a2966875
	github.com/mdlayher/ethernet v0.0.0-20220221185849-529eae5b6118
	github.com/mdlayher/packet v1.1.1
)

require (
	github.com/josharian/native v1.0.0 // indirect
	github.com/mdlayher/socket v0.4.0 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.2.0 // indirect
)

replace github.com/mdlayher/arp v0.0.0-20220512170110-6706a2966875 => github.com/networkop/arp v0.0.0-20230104202122-0d26934d4534
