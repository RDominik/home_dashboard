package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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
	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to encode payload:", err)
		os.Exit(1)
	}

	httpResp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read response:", err)
		os.Exit(1)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		fmt.Fprintln(os.Stderr, "request failed:", httpResp.Status)
		if len(respBody) > 0 {
			fmt.Fprintln(os.Stderr, string(respBody))
		}
		os.Exit(1)
	}

	var resp map[string]any
	if err := json.Unmarshal(respBody, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "failed to decode response:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))
}
