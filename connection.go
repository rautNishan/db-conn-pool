package dbconnpool

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Message pg sends
const (
	MsgAuthRequest     MessageType = 'R'
	ParameterStatus    MessageType = 'S'
	BackendKeyData     MessageType = 'K'
	ReadyForQuery      MessageType = 'Z'
	ErrorResponse      MessageType = 'E'
	EmptyQueryResponse MessageType = 'I'
	CommandComplete    MessageType = 'C'
	NoticeResponse     MessageType = 'N'
	RowDescription     MessageType = 'T'
	DataRow            MessageType = 'D'
)

type FieldRowDescriptor struct {
	Name         string
	TableOID     uint32
	ColumnAttr   uint16
	FieldOID     uint32
	DataTypeSize uint16
	TypeModifier uint32
	FormatCode   uint16
}

type Message struct {
	Type    byte
	Length  uint32
	Payload []byte
}
type MessageType byte
type AuthType uint32

// https://www.postgresql.org/docs/current/protocol-message-formats.html
const (
	AuthenticationOk                AuthType = 0
	AuthenticationKerberosV5        AuthType = 2
	AuthenticationCleartextPassword AuthType = 3
	AuthenticationMD5Password       AuthType = 5
	AuthenticationGSS               AuthType = 7
	AuthenticationGSSContinue       AuthType = 8
	AuthenticationSSPI              AuthType = 9
	AuthenticationSASL              AuthType = 10
	AuthenticationSASLContinue      AuthType = 11
	AuthenticationSASLFinal         AuthType = 12
)

type BeKeyData struct {
	ProcessID uint32
	SecretKey uint32
}
type Conn struct {
	txStatus       byte
	params         []string
	backendKeyData BeKeyData
	netConn        net.Conn //Only for testing
	release        func(healthy bool)
	timeOut        time.Time
}

// TODO fix overriding the timout for the active connection
func (conn *Conn) isAlive() bool {
	conn.netConn.SetDeadline(time.Now().Add(1 * time.Millisecond))
	defer conn.netConn.SetDeadline(time.Time{})
	buff := make([]byte, 1)
	_, err := conn.netConn.Read(buff)
	if err != nil {
		//if time out error then conn is stil alive
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Conn is Alive")
			return true
		}
		fmt.Println("Conn is not Alive")
		return false
	}
	return true
}

func (conn *Conn) getMessage() (Message, error) {
	header, err := conn.readExactly(5)

	if err != nil {
		return Message{}, err
	}
	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:5])
	payload, err := conn.readExactly(int(payloadLen) - 4) // length includes the 4-byte length field
	if err != nil {
		return Message{}, err
	}
	return Message{Type: msgType, Payload: payload, Length: payloadLen}, nil
}

func (conn *Conn) readExactly(n int) ([]byte, error) {
	buff := make([]byte, n)
	total := 0
	for total < n {
		read, err := conn.netConn.Read(buff[total:])
		if err != nil {
			return nil, err
		}
		total += read
	}
	return buff, nil
}

func (conn *Conn) Release() {
	if conn.txStatus == byte(EmptyQueryResponse) && conn.isAlive() {
		conn.release(true)
	} else {
		conn.release(false)
	}
}

func (conn *Conn) addTimeOuts(t time.Duration) {
	conn.timeOut = time.Now().Add(t)
}

func (conn *Conn) Query(query string) {
	msgLen := 4 + len(query) + 1     // length field itself + query + null terminator
	buf := make([]byte, 0, msgLen+1) //+1 for message type
	buf = append(buf, 'Q')
	buf = binary.BigEndian.AppendUint32(buf, uint32(msgLen))
	buf = append(buf, []byte(query)...)
	buf = append(buf, 0) // Null terminator
	_, err := conn.netConn.Write(buf)
	if err != nil {
		fmt.Println("Error in query: ", err)
	}
	conn.getQueryMessage()
}

func (conn *Conn) getQueryMessage() {
	for {
		fmt.Println("Again")
		msg, err := conn.getMessage()
		if err != nil {
			fmt.Println("Error while reading in getQuery message...", err)
			conn.release(false)
			return
		}
		fmt.Println("This is conn txstatus: ", string(conn.txStatus))
		switch msg.Type {
		case byte(RowDescription):
			fieldRowDescriptors, err := conn.parseRowDescriptor(msg.Payload)
			if err != nil {
				fmt.Println("This is error: ", err)
			}
			fmt.Println(fieldRowDescriptors)
		case byte(DataRow):
			conn.parseDataRow(msg.Payload)
		case byte(CommandComplete):
			conn.commandComplete(msg.Payload)
		case byte(ReadyForQuery):
			value := msg.Payload[0]
			conn.txStatus = value
			fmt.Println("Final txStatus:", string(conn.txStatus))

		}

	}
}

func (conn *Conn) parseRowDescriptor(payload []byte) ([]FieldRowDescriptor, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("Wrong payload")
	}
	nFields := binary.BigEndian.Uint16(payload[0:2])
	// fields := make([]FieldRowDescriptor, 0, nFields)
	offset := 2
	//For each fields we need to get the value
	var fields []FieldRowDescriptor
	for i := 0; i < int(nFields); i++ {
		//Get name
		nameStart := offset
		for offset < len(payload) && payload[offset] != 0 {
			offset++
		}
		name := string(payload[nameStart:offset])
		offset++ // skip null terminator
		//Now after name FieldRowDescriptor contains [int32(4 bytes), int16(2 bytes), int32 (4 bytes), int16 (2 bytes), int32(4 bytes), int16 (2 bytes)]
		remainingSize := 18
		if offset+remainingSize > len(payload) {
			return nil, fmt.Errorf("Wrong payload")
		}
		tableOid := binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
		colAttr := binary.BigEndian.Uint16(payload[offset : offset+2])
		offset += 2
		fieldOid := binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
		dataTypeSize := binary.BigEndian.Uint16(payload[offset : offset+2])
		offset += 2
		typeModifier := binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
		formatCode := binary.BigEndian.Uint16(payload[offset : offset+2])
		offset += 2
		field := FieldRowDescriptor{
			TableOID:     tableOid,
			Name:         name,
			ColumnAttr:   colAttr,
			FieldOID:     fieldOid,
			DataTypeSize: dataTypeSize,
			TypeModifier: typeModifier,
			FormatCode:   formatCode,
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func (conn *Conn) parseDataRow(payload []byte) {
	nCol := binary.BigEndian.Uint16(payload[:2])
	offSet := 2
	fmt.Println("Col number: ", nCol, "Offset: ", offSet)
	for i := 0; i < int(nCol); i++ {
		lenColVal := int32(binary.BigEndian.Uint32(payload[offSet : offSet+4])) //Need to cast it to int32 because we can get -1
		offSet += 4
		if (lenColVal) == -1 {
			fmt.Printf("col[%d]: NULL\n", i)
			continue
		}
		fmt.Println("Length of column value: ", lenColVal)
		val := payload[offSet : offSet+int(lenColVal)]
		offSet += int(lenColVal)
		fmt.Println("This is value: ", val)
	}
}

func (conn *Conn) commandComplete(payload []byte) {
	offset := 0
	for offset < len(payload) && payload[offset] != 0 {
		offset++
	}
	cmdTag := payload[:offset]
	offset += 1 //null terminator
	fmt.Println("Command tag: ", string(cmdTag))

}
