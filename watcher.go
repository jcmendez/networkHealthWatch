package main

import (
	"bytes"
	"fmt"
	"github.com/go-ping/ping"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	lastSuccess time.Time
	mutex       sync.Mutex
)

func checkICMPPing(ip string) bool {
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		fmt.Println("Error creating pinger:", err)
		return false
	}

	pinger.Count = 1
	pinger.Timeout = 2 * time.Second

	err = pinger.Run()
	if err != nil {
		fmt.Println("Error running pinger:", err)
		return false
	}

	stats := pinger.Statistics()
	return stats.PacketsRecv > 0
}

func turnSwitchOff(plugAddress, encryptionKey string) {

	// Define the JSON payload (empty in this case)
	payload := []byte("{}")

	// Create a new HTTP client
	client := &http.Client{}

	endpointTemplate := "http://{{address}}/api/services/switch/turn_on"
	endpoint := strings.ReplaceAll(endpointTemplate, "{{address}}", plugAddress)

	// Create a new HTTP request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error creating HTTP request:", err)
		return
	}

	// Add required headers (Content-Type and X-HA-Access for encryption)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HA-Access", encryptionKey)
	// Replace YOUR_ENCRYPTION_KEY with your actual encryption key

	// Send the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending HTTP request:", err)
		return
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Unexpected response status:", resp.Status)
		return
	}

	fmt.Println("Switch toggled successfully!")
}

func checkAndPost(plugToRestart, externalAddress, encryptionKey string) {
	for {
		// Check connection to the plug we would need to restart (the router, the cable box, etc.)
		if checkICMPPing(plugToRestart) {
			// If successful, check connection to external IP address (e.g. 8.8.8.8)
			if checkICMPPing(externalAddress) {
				// Connection to the outside is successful
				mutex.Lock()
				lastSuccess = time.Now()
				mutex.Unlock()
				break
			} else {
				// Connection to outside failed
				mutex.Lock()
				// Check if it's been more than 5 minutes since the last successful call
				if time.Since(lastSuccess) >= 5*time.Minute {
					// Perform HTTP POST
					turnSwitchOff(plugToRestart, encryptionKey)
					// Update last success time
					lastSuccess = time.Now()
				}
				mutex.Unlock()
				break
			}
		}
		// Sleep some time then try again
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
}

func main() {
	routerPlugAddress := "172.16.18.45"
	quadEightAddress := "8.8.8.8"
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		fmt.Println("Encryption key not set in environment variable.")
		return
	}

	// Initialize lastSuccess time
	lastSuccess = time.Now()

	go checkAndPost(routerPlugAddress, quadEightAddress, encryptionKey)

	// Keep the main goroutine running
	select {}
}
