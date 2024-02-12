package main

import (
	"bytes"
	"fmt"
	"github.com/go-ping/ping"
	"github.com/joho/godotenv"
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

// checkICMPPing sends an ICMP ping to the given IP address and returns true if it receives a reply.
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

// turnSwitchOff sends an HTTP POST request to the plug address to turn off the switch.
// It requires the encryption key as a header for authentication.
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

// checkAndPost checks the connectivity to the plug and the external address, and calls turnSwitchOff if needed.
// It runs in an infinite loop with a sleep interval of 1 second.
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

// main loads the encryption key from the environment variable and initializes the lastSuccess time.
// It then starts a goroutine to run checkAndPost and keeps the main goroutine running.
func main() {
	routerPlugAddress := "172.16.18.45"
	quadEightAddress := "8.8.8.8"

	// Try to load a .env file.  Otherwise, env vars can be provided
	godotenv.Load(".env")

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
