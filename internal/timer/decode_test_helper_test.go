package timer

import (
	"testing"

	"github.com/BurntSushi/toml"
)

// decodeTestPrimitive wraps a snippet of TOML fields as the body of a
// single [t] table and returns the resulting Primitive + MetaData, so
// DecodeTrigger/DecodeAction (which expect a table primitive) can be
// exercised directly in tests without going through internal/config.
func decodeTestPrimitive(t *testing.T, fields string) (toml.MetaData, toml.Primitive) {
	t.Helper()

	var doc struct {
		T toml.Primitive `toml:"t"`
	}
	src := "[t]\n" + fields
	meta, err := toml.Decode(src, &doc)
	if err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}
	return meta, doc.T
}
