package ids_test

import (
	"encoding/hex"
	"testing"

	. "parking-backend/internal/platform/ids"
)

func TestUUIDv7_VersionAndVariant(t *testing.T) {
	b := UUIDv7()
	if len(b) != 16 {
		t.Fatalf("len = %d, quiere 16", len(b))
	}
	if version := b[6] >> 4; version != 0x7 {
		t.Fatalf("versión = %x, quiere 7", version)
	}
	if variant := b[8] >> 6; variant != 0b10 {
		t.Fatalf("variante = %b, quiere 10", variant)
	}
}

func TestUUIDv7_StringRoundTrip(t *testing.T) {
	s := UUIDv7String()
	if len(s) != 32 {
		t.Fatalf("len = %d, quiere 32", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Fatalf("no es hex: %v", err)
	}

	b := StringToBin16(s)
	if len(b) != 16 {
		t.Fatalf("bin len = %d, quiere 16", len(b))
	}
	if got := Bin16ToString(b); got != s {
		t.Fatalf("roundtrip = %s, quiere %s", got, s)
	}
}

func TestStringToBin16_Invalid(t *testing.T) {
	if b := StringToBin16("no-es-hex"); b != nil {
		t.Fatalf("se esperaba nil, obtuvo %v", b)
	}
	if b := StringToBin16(""); b != nil {
		t.Fatalf("se esperaba nil para vacío, obtuvo %v", b)
	}
}

func TestStringToBin16_HyphenatedUUID(t *testing.T) {
	hyphenated := "018d3b5e-8842-7000-8000-123456789abc"
	b := StringToBin16(hyphenated)
	if len(b) != 16 {
		t.Fatalf("se esperaban 16 bytes, obtuvo %d", len(b))
	}
	expected := "018d3b5e884270008000123456789abc"
	if got := Bin16ToString(b); got != expected {
		t.Fatalf("Bin16ToString = %s, quiere %s", got, expected)
	}
}

func TestUUIDv7_TimeOrdered(t *testing.T) {
	first := UUIDv7()
	second := UUIDv7()
	if string(first[:6]) > string(second[:6]) {
		t.Fatal("los primeros 48 bits deben ser crecientes en el tiempo")
	}
}
