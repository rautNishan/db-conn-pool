package dbconnpool

import (
	"encoding/binary"
	"net"
)

// https://www.postgresql.org/docs/current/protocol-flow.html#PROTOCOL-FLOW-SIMPLE-QUERY
func SimpleQuery(query string, conn net.Conn) { //For testing purpose passing the connection itself
	msgLen := 4 + len(query) + 1     // length field itself + query + null terminator
	buf := make([]byte, 0, msgLen+1) //+1 for message type
	buf = append(buf, 'Q')
	buf = binary.BigEndian.AppendUint32(buf, uint32(msgLen))
	buf = append(buf, []byte(query)...)
	buf = append(buf, 0) // Null terminator
	conn.Write(buf)
}

func simpleQueryPipeline() {}
