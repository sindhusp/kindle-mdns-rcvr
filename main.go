package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
)

const mdnsPacketHeaderLength = 12

/** todo before release:
- make it a daemon
- what happens when the kindle sleeps?
- should this just be a binary we need to scp onto kindle, or should i try to make it a KUAL extension?
- logs / debugging?
- word about compiling
 */

func main() {
	interfaceName := flag.String("interface", "wlan0", "the interface you want to broadcast your IP to")
	domainName := flag.String("domain", "kindle.local", "the domain name for your kindle")
	flag.Parse()
	port := 5353
	ip := net.IPv4(224, 0, 0, 251)
	groupAddr := &net.UDPAddr {
		IP: ip,
		Port: port,
	}
	ifi, err := net.InterfaceByName(*interfaceName)
	if err != nil {
		fmt.Printf("no interface with this name %s", *interfaceName)
		//todo: error handling
	}

	conn, err := net.ListenMulticastUDP("udp4", ifi, groupAddr)
	if err != nil {
		log.Fatal("no conn established", err) // Todo: make a note of this in the docs
		//todo: error handling
	}
	buff := make([]byte, 65536)
	oob := make([]byte, 1024)
	for {
		n, _, _, _, err := conn.ReadMsgUDP(buff, oob)
		if err != nil {
			fmt.Println("couldn't read from the connection")
			continue
		}
		//decoded := make([]byte, n);
		//decodedStr := hex.Dump(decoded)

		fmt.Print(hex.Dump(buff[:n]))
		if mdnsPacketHeaderLength > n {
			fmt.Println("packet too short, skipping...")
			continue
		}
		header := buff[:mdnsPacketHeaderLength]
		//txnId := header[:2] //todo: if non-zero, its a unicast query, how to respond?
		qrBit := getBit(header[2], 7)
		if qrBit == 1 {
			// not a question
			fmt.Println("this packet is not a query, skipping...")
			continue
		}
		qCount := binary.BigEndian.Uint16(buff[4:6])

		names, _, err := parseNames(buff[:n], 12, qCount)
		if err != nil {
			fmt.Println("couldn't parse names. Skipping with err", err)
			continue
		}
		for _, name := range names {
			if !strings.EqualFold(name, *domainName) {
				fmt.Println("Querying someone else's name:", name)
				continue
			}

			ip, err = ipForInterface(ifi)
			if err != nil {
				fmt.Print("Couldn't find ip for interface, skipping") //todo: when will this actually happen, if ifi is null?
				continue
			}
			fmt.Println("Yes! Someone wants to know my IP!")
			resp := makeResponse(ip, *domainName)
			_, err := conn.WriteToUDP(resp, groupAddr)
			if err != nil {
				log.Println("failed to send response:", err)
			}
		}
	}
}

func makeResponse(ip net.IP, domainName string) [] byte {
	// mDNS responses set QR=1, AA=1, and — importantly — zero out the transaction ID.
	// Unlike unicast DNS, mDNS responses do NOT echo the query's ID; they use 0x0000.
	respHeader := make([]byte, 12)
	// resp[0], resp[1] = transaction ID = 0x0000 (already zero from make)
	respHeader[2] = 0x84 // flags high byte: QR=1 (0x80) + AA=1 (0x04)
	respHeader[3] = 0x00 // flags low byte
	// QDCOUNT = 0 (resp[4], resp[5]) — a response carries no questions
	// ANCOUNT = 1 (resp[6], resp[7]) — one answer record
	respHeader[7] = 0x01
	// NSCOUNT = 0, ARCOUNT = 0 (resp[8..11]) — already zero
	result := []byte{}
	//todo: make this static?

	result = append(result, respHeader...)
	result = append(result, encodeName(domainName)...)

	result = append(result, 0x00, 0x01)             // TYPE  = A (0x0001)
	result = append(result, 0x80, 0x01)             // CLASS = IN (0x0001) + cache-flush bit (0x8000)
	result = append(result, 0x00, 0x00, 0x00, 0x78) // TTL   = 120 seconds
	result = append(result, 0x00, 0x04)             // RDLENGTH = 4 (an IPv4 addr is 4 bytes)
	result = append(result, ip[0], ip[1], ip[2], ip[3]) // RDATA = the 4 IP bytes
	return result
}

func ipForInterface(ifi *net.Interface) (net.IP, error) {
	addrs, err := ifi.Addrs() // all addresses on this interface
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok { // addresses come as *net.IPNet
			if ip4 := ipnet.IP.To4(); ip4 != nil { // want IPv4, skip IPv6
				return ip4, nil
			}
		}
	}
	return nil, fmt.Errorf("no IPv4 address on %s", ifi.Name)
}

func encodeName(name string) []byte {
	result := []byte{}                  // start empty, no guessed size
	for _, label := range strings.Split(name, ".") {
		result = append(result, byte(len(label))) // length octet
		result = append(result, []byte(label)...) // the label's bytes
	}
	result = append(result, 0x00)       // ONE terminator, after the loop
	return result
}

/**
- to implement compressed pointers
1. understand when the byte is a compressed ptr. first 2 bits are id.
whole msg incl id will be 2 bytes. 2 * 8 -> 16 bits. so 14 bits to rep offset space. offset starts from the beginning of the msg.
keep a map of label read so far to offset start.

3 formats to consider
- whole thing is a ptr -> 11 25 -> im a label ptr, im asking for the thing at 25. whats at 25? length followed by labels till root.
- here's the issue: the whole thing needs a stack i think. whenever a label is id'd it needs to be mapped. but the whole domain name needs to be mapped as well
- its a set of labels followed by a ptr
- its a set of labels followed by root. (handled)
*/
func parseNames(buf []byte, offset int, qCount uint16) ([]string, int, error) {
	var names []string
	for qCount > 0 {
		name, nextOffset, err := parseName(buf, offset)
		if err != nil {
			return nil, 0, err
		}
		names = append(names, name)
		qCount--
		offset = nextOffset + 4
	}
	return names, offset, nil
}

func parseName(buf []byte, offset int) (string, int, error) {
	i := offset
	var labels []string
	for {
		if i >= len(buf) {
			return "", 0, fmt.Errorf("name overran buffer at %d", i)
		}
		length := int(buf[i])
		if length == 0 {
			i++
			break
		}
		if length >= 0xC0 { // top two bits set = compression pointer.
			// fixme: do exact 2 bit eq comparison for forward compatibility
			if i+2 > len(buf) {
				return "", 0, fmt.Errorf("name overran buffer at %d", i)
			}
			ptrOffset := binary.BigEndian.Uint16(buf[i:i+2])
			ptrOffset = ptrOffset & 0x3FFF
			if int(ptrOffset) >= offset { // must point strictly backward
				return "", 0, fmt.Errorf("non-backward pointer %d", ptrOffset)
			}
			suffix, _, err := parseName(buf, int(ptrOffset))
			if err != nil {
				return "", 0, err
			}
			name := strings.Join(append(labels, suffix), ".")
			fmt.Println("I see a compressed name! ", name)
			return name, i + 2, nil
		}
		i++
		end := i + length
		if end > len(buf) {
			return "", 0, fmt.Errorf("label len %d overruns buffer", length)
		}
		thisLabel := string(buf[i:end])
		labels = append(labels, thisLabel)
		i = end
	}

	return strings.Join(labels, "."), i, nil
}

func getBit(b byte, position int) int {
	// Shift target bit to the lowest position and mask with 1
	return int((b >> position) & 1)
}
