package dbconnpool

import "encoding/binary"

func StartUp(user, database string) []byte {
	// A zero byte is required as a terminator after the last name/value pair. (https://www.postgresql.org/docs/current/protocol-message-formats.html)
	// Parameters can appear in any order. user is required, others are optional. Each parameter is specified as:
	payload := []byte(
		"user\x00" + user + "\x00" +
			"database\x00" + database + "\x00" + "\x00",
	)
	total := uint32(4 + 4 + len(payload))
	version := uint32(0x00030000) //The protocol version number.
	//  The most significant 16 bits are the major version number
	// (3 for the protocol described here). The least significant
	//  16 bits are the minor version number (2 for the protocol described here).
	buffer := make([]byte, 0, total)
	buffer = binary.BigEndian.AppendUint32(buffer, total)
	buffer = binary.BigEndian.AppendUint32(buffer, version)
	buffer = append(buffer, payload...)
	return buffer
}
