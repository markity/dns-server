package main

import (
	"net"
	"strings"
)

func main() {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53})
	if err != nil {
		panic(err)
	}

	bs := make([]byte, 3000)
	for {
		n, addr, err := c.ReadFromUDP(bs)
		if err != nil {
			panic(err)
		}

		pack := bs[:n]

		req, ok := ParseRequestPacket(pack)
		if !ok {
			panic("unexpected")
		}

		if req.Query != QueryTypeNormal || len(req.Questions) != 1 ||
			(req.Questions[0].QuestionType != QuestionTypeA &&
				req.Questions[0].QuestionType != QuestionTypeAAAA) {
			println("不支持", req.Questions[0].QuestionType == QuestionTypeAAAA)
			_, err := c.WriteToUDP(MakeBytesNoEntry(req), addr)
			if err != nil {
				panic(err)
			}
			// 不支持
			continue
		}

		name := strings.Join(req.Questions[0].Question, ".")
		if name != "my-service.com" {
			println("no entry")
			_, err := c.WriteToUDP(MakeBytesUnsupportedQuery(req.TX), addr)
			if err != nil {
				panic(err)
			}
			continue
		}

		if req.Questions[0].QuestionType == QuestionTypeA {
			println("ipv4")
			_, err = c.WriteToUDP(MakeBytesResponseSigleIPV4(req.TX, req.Questions[0].Question, 3000, 127, 0, 0, 1), addr)
			if err != nil {
				panic(err)
			}
		} else {
			println("ipv6")
			_, err = c.WriteToUDP(MakeBytesNoEntry(req), addr)
			if err != nil {
				panic(err)
			}
		}
	}
}
