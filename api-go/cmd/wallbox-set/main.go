package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"webgui-api/rest"
)

func parseValue(s string) any {
	// try to decode as JSON (number, bool, object, array)
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	// fallback to string
	return s
}

func main() {
	var key string
	var value string
	var url string

	flag.StringVar(&key, "key", "", "wallbox key (e.g. amp, frc, psm, dwo)")
	flag.StringVar(&value, "value", "", "value to send (plain number, JSON or string)")
	flag.StringVar(&url, "url", "http://localhost:8083/api/wallbox/set", "API url")
	flag.Parse()

	if key == "" {
		fmt.Fprintln(os.Stderr, "--key is required")
		os.Exit(2)
	}

	payload := map[string]any{"key": key, "value": parseValue(value)}
	var resp map[string]any
	if err := rest.PostJSON(url, payload, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))
}
