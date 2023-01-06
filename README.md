# rarp-replay

Implementation of ARP/ND probe from https://datatracker.ietf.org/doc/html/draft-ietf-bess-evpn-irb-extended-mobility-08#section-8.8

## Requirements

```
sudo apt install libpcap0.8-dev
```

## Run RARP request

```
sudo ./rarp-req -mac 44:38:39:22:01:23 -intf eth0
```