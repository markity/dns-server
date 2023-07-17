package main

import (
	"encoding/binary"
)

type QueryType int

const (
	// 正常查询
	QueryTypeNormal QueryType = 0
	// 反向查询
	QueryTypeReverse = 1
	// 服务器状态请求
	QueryTypeState = 2
)

type QuestionType int

const (
	QuestionTypeA QuestionType = iota
	QuestionTypeAAAA
	// 我们不关心除了A和AAAA的其它记录
	QuestsionTypeOther
)

type ClassType int

const (
	ClassTypeInternet ClassType = iota
	// 我们不关心除了A记录的其它记录
	ClassTypeOther
)

type Question struct {
	QuestionType QuestionType
	Question     []string
	ClassType    ClassType
}

type RequestPacketInfo struct {
	// 事务ID
	TX uint16
	//
	Query        QueryType
	RecurDisired bool
	Questions    []*Question
}

func ParseRequestPacket(bs []byte) (*RequestPacketInfo, bool) {
	if len(bs) < 12 {
		return nil, false
	}

	packHeader := bs[:12]
	tx := binary.BigEndian.Uint16(packHeader[0:2])
	flag := binary.BigEndian.Uint16(packHeader[2:4])
	que := binary.BigEndian.Uint16(packHeader[4:6])
	ans := binary.BigEndian.Uint16(packHeader[6:8])
	authNum := binary.BigEndian.Uint16(packHeader[8:10])
	addNum := binary.BigEndian.Uint16(packHeader[10:12])

	flagQR := flag & 1                        // 第一个字节
	flagOpcode := (flag & (0b1111 << 1)) >> 1 // 第2 3 4 5个字节
	// flagAA := (flag & (0b1 << 5)) >> 5        // 第6个字节, 这个不用管
	flagTC := (flag & (0b1 << 6)) >> 6 // 第7个字节
	flagRD := (flag & (0b1 << 7)) >> 7 // 第8个字节
	// flagRA := (flag & (0b1 << 8)) >> 8          // 第9个字节, 这个不用管
	flagZero := (flag & (0b111 << 9)) >> 9 // 第10 11 12个字节
	// flagRecord := (flag & (0b1111 << 12)) >> 12 // 第13 14 15 16个字节, 这个不用管

	// QR代表是否是response
	if flagQR != 0 {
		return nil, false
	}

	var queryType QueryType
	switch flagOpcode {
	case 0:
		queryType = QueryTypeNormal
	case 1:
		queryType = QueryTypeReverse
	case 2:
		queryType = QueryTypeState
	default:
		return nil, false
	}

	// TODO: 截断怎么操作?
	if flagTC == 1 {
		return nil, false
	}

	var recurDisired bool = false
	if flagRD == 1 {
		recurDisired = true
	}

	// 标准规定必须为0
	if flagZero != 0 {
		return nil, false
	}

	// 对于请求, 只允许有查询区域, 没有其它任何区域
	if que == 0 {
		return nil, false
	}
	if ans != 0 {
		return nil, false
	}
	if authNum != 0 {
		return nil, false
	}
	if addNum != 0 {
		return nil, false
	}

	payloadArea := bs[12:]
	if len(payloadArea) == 0 {
		return nil, false
	}

	// 读取问题
	ques := make([]*Question, 0)
	for i := 0; i < int(que); i++ {
		curQuestionSlice := make([]string, 0)

		// 先拿第一个字节, 查看第一个节的长度
		p := &PeekReader{
			Data: payloadArea,
			Pos:  0,
			Len:  len(payloadArea),
		}

		firstByte, ok := p.Move()
		if !ok {
			return nil, false
		}
		numByte := uint8(firstByte)

		if numByte == 0 {
			return nil, false
		}

		builder := make([]byte, 0, numByte)
		for i := uint8(0); i < numByte; i++ {
			b, ok := p.Move()
			if !ok {
				return nil, false
			}
			builder = append(builder, b)
		}
		curQuestionSlice = append(curQuestionSlice, string(builder))

		for {
			firstByte, ok := p.Move()
			if !ok {
				return nil, false
			}
			numByte := uint8(firstByte)

			if numByte == 0 {
				break
			}

			builder := make([]byte, 0, numByte)
			for i := uint8(0); i < numByte; i++ {
				b, ok := p.Move()
				if !ok {
					return nil, false
				}
				builder = append(builder, b)
			}
			curQuestionSlice = append(curQuestionSlice, string(builder))
		}

		// 拿记录类型
		if p.IsEnd() {
			return nil, false
		}
		extraBytes := payloadArea[p.Pos:]
		if len(extraBytes) != 4 {
			return nil, false
		}

		var questionType QuestionType
		switch binary.BigEndian.Uint16(extraBytes) {
		case 1:
			questionType = QuestionTypeA
		case 28:
			questionType = QuestionTypeAAAA
		default:
			questionType = QuestsionTypeOther
		}

		var classType ClassType
		switch binary.BigEndian.Uint16(extraBytes[2:]) {
		case 1:
			classType = ClassTypeInternet
		default:
			classType = ClassTypeOther
		}

		curQuestion := Question{
			QuestionType: questionType,
			Question:     curQuestionSlice,
			ClassType:    classType,
		}
		ques = append(ques, &curQuestion)
	}

	return &RequestPacketInfo{TX: tx, Query: queryType, RecurDisired: recurDisired, Questions: ques}, true
}

func MakeBytesUnsupportedQuery(tx uint16) []byte {
	bs := make([]byte, 0)
	bs = binary.BigEndian.AppendUint16(bs, tx)

	flag := uint16(0)
	flag |= 1 << 15
	flag |= 4
	bs = binary.BigEndian.AppendUint16(bs, flag) // flag

	bs = binary.BigEndian.AppendUint16(bs, 0) // 问题数=1
	bs = binary.BigEndian.AppendUint16(bs, 0) // 答案数=1
	bs = binary.BigEndian.AppendUint16(bs, 0) // 授权服务器数=0
	bs = binary.BigEndian.AppendUint16(bs, 0) // 附加信息数=0

	return bs
}

func MakeBytesNoEntry(req *RequestPacketInfo) []byte {
	bs := make([]byte, 0)
	bs = binary.BigEndian.AppendUint16(bs, req.TX)

	flag := uint16(0)
	flag |= 1 << 15
	flag |= 3
	bs = binary.BigEndian.AppendUint16(bs, flag) // flag

	bs = binary.BigEndian.AppendUint16(bs, 1) // 问题数=1
	bs = binary.BigEndian.AppendUint16(bs, 0) // 答案数=0
	bs = binary.BigEndian.AppendUint16(bs, 0) // 授权服务器数=0
	bs = binary.BigEndian.AppendUint16(bs, 0) // 附加信息数=0

	for _, v := range req.Questions[0].Question {
		bs = append(bs, byte(len(v)))
		bs = append(bs, v...)
	}
	bs = append(bs, 0)

	if req.Questions[0].QuestionType == QuestionTypeAAAA {
		bs = binary.BigEndian.AppendUint16(bs, 28) // type = 1
	} else {
		bs = binary.BigEndian.AppendUint16(bs, 1) // type = 1
	}
	bs = binary.BigEndian.AppendUint16(bs, 1) // type = 1
	bs = binary.BigEndian.AppendUint16(bs, 1) // class = 1

	return bs
}

func MakeBytesResponseSigleIPV4(tx uint16, questsion []string, ttl int, ipb1 byte,
	ipb2 byte, ipb3 byte, ipb4 byte) []byte {
	bs := make([]byte, 0)
	bs = binary.BigEndian.AppendUint16(bs, tx)

	flag := uint16(0)
	flag |= 1 << 15                              // QR = 1
	bs = binary.BigEndian.AppendUint16(bs, flag) // flag

	bs = binary.BigEndian.AppendUint16(bs, 1) // 问题数=1
	bs = binary.BigEndian.AppendUint16(bs, 1) // 答案数=1
	bs = binary.BigEndian.AppendUint16(bs, 0) // 授权服务器数=0
	bs = binary.BigEndian.AppendUint16(bs, 0) // 附加信息数=0

	// 写问题
	for _, v := range questsion {
		bs = append(bs, byte(len(v)))
		bs = append(bs, []byte(v)...)
	}
	bs = append(bs, 0)
	bs = binary.BigEndian.AppendUint16(bs, 1) // type = 1
	bs = binary.BigEndian.AppendUint16(bs, 1) // class = 1

	d := uint16(0)
	d |= 0b11 << 14
	d |= 12
	bs = binary.BigEndian.AppendUint16(bs, d)
	bs = binary.BigEndian.AppendUint16(bs, 1)           // type = 1
	bs = binary.BigEndian.AppendUint16(bs, 1)           // class = 1
	bs = binary.BigEndian.AppendUint32(bs, uint32(ttl)) // ttl
	bs = binary.BigEndian.AppendUint16(bs, 4)
	bs = append(bs, 127)
	bs = append(bs, 0)
	bs = append(bs, 0)
	bs = append(bs, 1)

	return bs
}

type PeekReader struct {
	Data []byte
	Pos  int
	Len  int
}

func (p *PeekReader) Move() (byte, bool) {
	if p.Pos == p.Len {
		return 0, false
	}

	prePos := p.Pos
	p.Pos++
	return p.Data[prePos], true
}

func (p *PeekReader) Peek() (byte, bool) {
	if p.Pos == p.Len {
		return 0, false
	}

	return p.Data[p.Pos], true
}

func (p *PeekReader) IsEnd() bool {
	return p.Pos == p.Len
}

type Response struct {
}
