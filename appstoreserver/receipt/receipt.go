// Package receipt extracts transaction IDs to migrate legacy receipts.
// It does not authenticate receipts. Use IDs only to query App Store Server API,
// then verify the returned signed transaction before granting entitlements.
package receipt

import (
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"regexp"
)

type ReceiptUtility struct{}

func (ReceiptUtility) ExtractTransactionIDFromAppReceipt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	root, rest, err := readBER(data, 0)
	if err != nil || len(rest) != 0 || root.tag != 0x30 {
		return "", errors.New("invalid PKCS7 receipt")
	}
	children, err := root.children()
	if err != nil || len(children) < 2 {
		return "", errors.New("invalid PKCS7 content info")
	}
	var oid asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(children[0].encoded, &oid); err != nil || !oid.Equal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}) {
		return "", errors.New("receipt is not PKCS7 SignedData")
	}
	wrapped, err := children[1].children()
	if err != nil || len(wrapped) != 1 {
		return "", errors.New("invalid SignedData wrapper")
	}
	signedData, err := wrapped[0].children()
	if err != nil || len(signedData) < 3 {
		return "", errors.New("invalid SignedData")
	}
	contentInfo, err := signedData[2].children()
	if err != nil || len(contentInfo) < 2 {
		return "", errors.New("receipt has no content")
	}
	content, err := contentInfo[1].children()
	if err != nil || len(content) != 1 {
		return "", errors.New("invalid receipt payload")
	}
	payload, err := octets(content[0])
	if err != nil {
		return "", err
	}
	attributes, err := receiptAttributes(payload)
	if err != nil {
		return "", err
	}
	for _, attribute := range attributes {
		kind, body, err := attributeValue(attribute)
		if err != nil {
			return "", err
		}
		if kind != 17 {
			continue
		}
		purchases, err := receiptAttributes(body)
		if err != nil {
			return "", err
		}
		for _, purchase := range purchases {
			kind, value, err := attributeValue(purchase)
			if err != nil {
				return "", err
			}
			if kind == 1703 || kind == 1705 {
				var id string
				rest, err := asn1.Unmarshal(value, &id)
				if err != nil || len(rest) != 0 {
					return "", errors.New("invalid receipt transaction ID")
				}
				return id, nil
			}
		}
	}
	return "", nil
}

var purchaseInfo = regexp.MustCompile(`"purchase-info"\s+=\s+"([a-zA-Z0-9+/=]+)";`)
var transactionID = regexp.MustCompile(`"transaction-id"\s+=\s+"([a-zA-Z0-9+/=]+)";`)

func (ReceiptUtility) ExtractTransactionIDFromTransactionReceipt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	match := purchaseInfo.FindSubmatch(data)
	if len(match) != 2 {
		return "", errors.New("receipt has no purchase-info")
	}
	purchase, err := base64.StdEncoding.DecodeString(string(match[1]))
	if err != nil {
		return "", err
	}
	match = transactionID.FindSubmatch(purchase)
	if len(match) != 2 {
		return "", nil
	}
	return string(match[1]), nil
}

type berValue struct {
	tag           byte
	data, encoded []byte
	depth         int
}

// Receipts may contain BER indefinite-length constructed OCTET STRINGs, which
// encoding/asn1's DER decoder cannot read. Bound nesting while parsing wrappers.
func readBER(data []byte, depth int) (berValue, []byte, error) {
	if depth > 32 || len(data) < 2 || data[0]&31 == 31 {
		return berValue{}, nil, errors.New("invalid BER receipt")
	}
	tag := data[0]
	position := 2
	length := int(data[1])
	if length == 128 {
		if tag&32 == 0 {
			return berValue{}, nil, errors.New("invalid primitive indefinite length")
		}
		start := position
		for {
			if len(data)-position < 2 {
				return berValue{}, nil, errors.New("unterminated BER value")
			}
			if data[position] == 0 && data[position+1] == 0 {
				return berValue{tag, data[start:position], data[:position+2], depth}, data[position+2:], nil
			}
			_, rest, err := readBER(data[position:], depth+1)
			if err != nil {
				return berValue{}, nil, err
			}
			position = len(data) - len(rest)
		}
	}
	if length > 128 {
		count := length & 127
		if count > 8 || len(data)-position < count {
			return berValue{}, nil, errors.New("invalid BER length")
		}
		length = 0
		for i := 0; i < count; i++ {
			if length > (len(data) >> 8) {
				return berValue{}, nil, errors.New("BER length exceeds receipt")
			}
			length = length*256 + int(data[position])
			position++
		}
	}
	if length > len(data)-position {
		return berValue{}, nil, errors.New("truncated BER receipt")
	}
	end := position + length
	return berValue{tag, data[position:end], data[:end], depth}, data[end:], nil
}
func (v berValue) children() ([]berValue, error) {
	if v.tag&32 == 0 {
		return nil, errors.New("expected constructed receipt value")
	}
	var result []berValue
	for data := v.data; len(data) > 0; {
		child, rest, err := readBER(data, v.depth+1)
		if err != nil {
			return nil, err
		}
		result = append(result, child)
		data = rest
	}
	return result, nil
}
func octets(v berValue) ([]byte, error) {
	if v.tag == 4 {
		return v.data, nil
	}
	if v.tag != 0x24 {
		return nil, errors.New("expected receipt octet string")
	}
	children, err := v.children()
	if err != nil {
		return nil, err
	}
	var data []byte
	for _, child := range children {
		part, err := octets(child)
		if err != nil {
			return nil, err
		}
		data = append(data, part...)
	}
	return data, nil
}
func receiptAttributes(data []byte) ([]berValue, error) {
	root, rest, err := readBER(data, 0)
	if err != nil || len(rest) != 0 || root.tag != 0x31 {
		return nil, errors.New("invalid receipt attribute set")
	}
	return root.children()
}
func attributeValue(v berValue) (int, []byte, error) {
	if v.tag != 0x30 {
		return 0, nil, errors.New("invalid receipt attribute")
	}
	fields, err := v.children()
	if err != nil || len(fields) != 3 {
		return 0, nil, errors.New("invalid receipt attribute sequence")
	}
	var kind int
	if _, err := asn1.Unmarshal(fields[0].encoded, &kind); err != nil {
		return 0, nil, err
	}
	data, err := octets(fields[2])
	return kind, data, err
}
