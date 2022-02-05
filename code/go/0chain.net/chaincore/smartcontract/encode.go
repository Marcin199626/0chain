package smartcontract

import "encoding/json"

// Encode encode smart contract
func Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Decode decodes the smart contract data
func Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
