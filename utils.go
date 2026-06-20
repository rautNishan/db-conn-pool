package dbconnpool

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

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

func parseSaslMechanism(payload []byte) []string {
	var mecha []string
	start := 0
	end := 0
	for end < len(payload) {
		if payload[end] == 0 {
			if start < end {
				mecha = append(mecha, string(payload[start:end]))
			}
			end++ //skip null terminator
			start = end
		}
		end++
	}
	return mecha
}

// https://www.postgresql.org/docs/current/sasl-authentication.html
// https://datatracker.ietf.org/doc/html/rfc5802#section-3
// https://datatracker.ietf.org/doc/html/rfc5802#section-5
func buildClientFirstMessage(username string, tlsEnabled bool) (string, string, string, error) {
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	clientNonce := base64.StdEncoding.EncodeToString(nonceBytes)
	clientFirstMessageBare := fmt.Sprintf("n=%s,r=%s", username, clientNonce)
	var gs2Header string
	if tlsEnabled {
		gs2Header = "y,,"
	} else {
		gs2Header = "n,,"
	}
	clientFirstMessage := gs2Header + clientFirstMessageBare

	return clientFirstMessage, clientFirstMessageBare, clientNonce, nil
}
