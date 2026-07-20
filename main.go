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
	//ifi, err := net.InterfaceByName("wlan0")
	//if err != nil {
	//	log.Fatal(err);
	//	todo: error handling
	//}
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
		//qcount := buff[4:6]
		var indexQEnd int

		for i, b := range buff[12:] {
			if b == 0x00 {
				indexQEnd = i
				break
			}
		}

		question := buff[12:12+indexQEnd]

		//FIXME: Stopping at 0 byte will not work for dns-compressed packets. Impl with length bytes
		//lengthByte := buff[12:13]

		fmt.Println("this packet is querying ", hex.Dump(question))
		if strings.EqualFold(string(hex.Dump(question)), myQuestion) {
			fmt.Println("Yes! Someone wants to know my IP!")
		} else {
			fmt.Println(string(question))
		}
	}
}

func getBit(b byte, position int) int {
	// Shift target bit to the lowest position and mask with 1
	return int((b >> position) & 1)
}