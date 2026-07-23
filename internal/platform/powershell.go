package platform

import (
	"encoding/base64"
	"unicode/utf16"
)

// EncodePowerShellCommand returns a Base64-encoded UTF-16 LE representation of
// the script, which is what Windows PowerShell's -EncodedCommand parameter
// expects.
func EncodePowerShellCommand(script string) (string, error) {
	u16 := utf16.Encode([]rune(script))
	buf := make([]byte, len(u16)*2)
	for i, r := range u16 {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
