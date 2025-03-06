package helpers

import (
	"encoding/base64"
)

// EncodePagingState encodes a paging state (byte slice) into a Base64 string.
func EncodePagingState(pagingState []byte) *string {
	if len(pagingState) == 0 {
		return nil
	}
	encoded := base64.StdEncoding.EncodeToString(pagingState)
	return &encoded
}

// DecodePagingState decodes a Base64 paging state string into a byte slice.
func DecodePagingState(pagingState *string) []byte {
	if pagingState == nil || *pagingState == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(*pagingState)
	if err != nil {
		return nil
	}
	return decoded
}
