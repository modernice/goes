package main

import (
	"bytes"
	"os"
	"testing"
)

func TestMainPrintsRenamedProduct(t *testing.T) {
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	main()

	_ = w.Close()
	os.Stdout = stdout
	output := <-done

	if output != "Ergonomic Wireless Mouse\n" {
		t.Fatalf("unexpected output: %q", output)
	}
}
