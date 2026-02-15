package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const defaultLuxPort = "19419"

func luxBaseURL() string {
	port := os.Getenv("LUX_PORT")
	if port == "" {
		port = defaultLuxPort
	}
	return fmt.Sprintf("http://localhost:%s", port)
}

func postToLux(path string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(luxBaseURL()+path, "application/json", bytes.NewReader(data))
	if err != nil {
		// Fail open: lux not running is fine
		return nil
	}
	defer resp.Body.Close()

	return nil
}
