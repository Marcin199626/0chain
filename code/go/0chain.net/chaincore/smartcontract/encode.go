package smartcontract

import (
	"encoding/json"

	"0chain.net/core/util"
)

// Encode encode smart contract
func Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Decode decodes the smart contract data
func Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// EncodeTxnOutput encodes the smart contract output node bytes into hex
func EncodeTxnOutput(v util.Serializable) string {
	return util.ToHex(v.Encode())
}
