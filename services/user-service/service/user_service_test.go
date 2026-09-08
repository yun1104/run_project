package service

import "testing"

func TestUserServiceHashPassword(t *testing.T) {
	got := hashPassword("123456")
	if got == "" {
		t.Fatal("hash should not be empty")
	}
	if got == "123456" {
		t.Fatal("hash should not equal plain password")
	}
	if got != hashPassword("123456") {
		t.Fatal("same password should produce same hash")
	}
	if got == hashPassword("abcdef") {
		t.Fatal("different passwords should produce different hashes")
	}
}

func TestMarshalAndUnmarshalArrays(t *testing.T) {
	stringJSON := marshalArray([]string{"川菜", "快餐"})
	strings := unmarshalStringArray(stringJSON)
	if len(strings) != 2 || strings[0] != "川菜" || strings[1] != "快餐" {
		t.Fatalf("string array round trip mismatch: %+v", strings)
	}

	int64JSON := marshalArray([]int64{10001, 10002})
	int64s := unmarshalInt64Array(int64JSON)
	if len(int64s) != 2 || int64s[0] != 10001 || int64s[1] != 10002 {
		t.Fatalf("int64 array round trip mismatch: %+v", int64s)
	}

	intJSON := marshalArray([]int{12, 18})
	ints := unmarshalIntArray(intJSON)
	if len(ints) != 2 || ints[0] != 12 || ints[1] != 18 {
		t.Fatalf("int array round trip mismatch: %+v", ints)
	}
}

func TestUnmarshalInvalidArraysReturnsEmpty(t *testing.T) {
	if got := unmarshalStringArray("not-json"); len(got) != 0 {
		t.Fatalf("invalid string json should return empty slice, got %+v", got)
	}
	if got := unmarshalInt64Array("not-json"); len(got) != 0 {
		t.Fatalf("invalid int64 json should return empty slice, got %+v", got)
	}
	if got := unmarshalIntArray("not-json"); len(got) != 0 {
		t.Fatalf("invalid int json should return empty slice, got %+v", got)
	}
}
