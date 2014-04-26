package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func ExampleEncodeKey() {
	type T struct {
		X uint16
		Y uint16
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, T{X: 42, Y: 10000}); err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	buf.WriteString("test")
	key := buf.Bytes()
	filename := EncodeKey(key)
	fmt.Println(filename)
	// Output:
	// @002a2710:test
}
