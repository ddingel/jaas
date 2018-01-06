package main

import "testing"
import "os"

func TestEmptyArguments(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Missing panic for missing arguments")
		}
	}()

	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	os.Args = []string{"cmd"}
	parseAndValidate()
}

func TestImageArgument(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = []string{"cmd", "hello-world"}
	if jaasCmd := parseAndValidate(); jaasCmd.Image != "hello-world" {
		t.Errorf("Parsing image failed")
	}
}
