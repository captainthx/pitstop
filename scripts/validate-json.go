package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	var v any
	d := json.NewDecoder(os.Stdin)
	if err := d.Decode(&v); err != nil {
		panic(err)
	}
	if err := d.Decode(&v); err != io.EOF {
		panic("multiple documents")
	}
}
