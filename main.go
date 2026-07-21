package main;

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
)

const mdnsPacketHeaderLength = 12
const myQuestion = "kindle.local"
/**
1. join mdns broadcast group
2. print every package received
 */
func main() {
	fmt.Println("Hello")
	port := 5353
	ip := net.IPv4(224, 0, 0, 251)
	groupAddr := &net.UDPAddr {
		IP: ip,
		Port: port,
	}
	ifi, err := net.InterfaceByName("en0")
	if err != nil {
		fmt.Println("no interface with this name")
		//todo: error handling
	}
	//fixme:
	// before running this on kindle, verify that wifi network is called wlan0.
	// make it a flag so if others find a problem, they can run ifconfig on their kindle and replace this param
	conn, err := net.ListenMulticastUDP("udp4", nil, groupAddr)
	if err != nil {
		log.Fatal(err);
		//todo: error handling
	}
	buff := make([]byte, 65536);
	oob := make([]byte, 65536); //fixme: overallocation but what is the right number?
	for {
		n, _, _, _, err := conn.ReadMsgUDP(buff, oob)
		if (err != nil) {
			log.Fatal(err);
		}
		//decoded := make([]byte, n);
		//decodedStr := hex.Dump(decoded)

		fmt.Printf(hex.Dump(buff[:n]))

		header := buff[:mdnsPacketHeaderLength]
		//txnId := header[:2] //todo: if non-zero, its a unicast query, how to respond?
		qrBit := getBit(header[2], 7)
		if (qrBit == 1) {
			// not a question
			fmt.Println("this packet is not a query, skipping...")
			continue
		}
		////qcount := buff[4:6]
		//var indexQEnd int
		//
		//for i, b := range buff[12:] {
		//	if b == 0x00 {
		//		indexQEnd = i
		//		break
		//	}
		//}
		//
		//question := buff[12:12+indexQEnd]

		//lengthByte := buff[12:13]
		name, _, _ := parseName(buff, 12);
		fmt.Println("this packet is querying ", name)
		ip, err = ipForInterface(ifi)
		fmt.Println("response: ", makeResponse(ip))

		if strings.EqualFold(name, myQuestion) {
			fmt.Println("Yes! Someone wants to know my IP!")
			resp := makeResponse(ip)
			_, err := conn.WriteToUDP(resp, groupAddr)
			if err != nil {
				log.Println("failed to send response:", err)
			}
		} else {
			fmt.Println("Querying someone else's name:", name)
		}
	}
}

func makeResponse(ip net.IP) [] byte {
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
	result = append(result, encodeName(myQuestion)...)

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

//TODO: compressed ptrs not implemented yet
//TODO: multiple queries in the same packet not implemented yet
func parseName(buf []byte, offset int) (string, int, error) {
	var labels []string
	i := offset
	for {
		if i >= len(buf) {
			return "", 0, fmt.Errorf("name overran buffer at %d", i)
		}
		length := int(buf[i]) // the length octet
		i++                   // step over the length octet itself

		if length == 0 { // zero-length octet = root label = name is done
			break
		}
		if length >= 0xC0 { // top two bits set = compression pointer
			return "", 0, fmt.Errorf("compression pointer TODO")
			// later: 14-bit offset from these 2 bytes, jump there, but leave i just past them
		}
		end := i + length
		if end > len(buf) {
			return "", 0, fmt.Errorf("label len %d overruns buffer", length)
		}
		labels = append(labels, string(buf[i:end])) // the label's bytes
		i = end                                      // advance past this label
	}
	return strings.Join(labels, "."), i, nil
}

func getBit(b byte, position int) int {
	// Shift target bit to the lowest position and mask with 1
	return int((b >> position) & 1)
}