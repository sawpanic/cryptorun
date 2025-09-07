package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	fmt.Println("🔍 Debugging Kraken API Direct Call")
	
	ctx := context.Background()
	
	// Test direct HTTP call to Kraken
	url := "https://api.kraken.com/0/public/Depth?pair=XXBTZUSD&count=5"
	
	fmt.Printf("Testing URL: %s\n", url)
	
	client := &http.Client{Timeout: 15 * time.Second}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("❌ Failed to create request: %v\n", err)
		return
	}
	
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CryptoRun/1.0")
	
	fmt.Println("Making request...")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	fmt.Printf("Status: %s\n", resp.Status)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Failed to read body: %v\n", err)
		return
	}
	
	if resp.StatusCode != 200 {
		fmt.Printf("❌ HTTP Error %d: %s\n", resp.StatusCode, string(body))
		return
	}
	
	// Pretty print JSON response
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		fmt.Printf("❌ Failed to parse JSON: %v\n", err)
		fmt.Printf("Raw response: %s\n", string(body))
		return
	}
	
	prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
	fmt.Printf("✅ Success! Response:\n%s\n", string(prettyJSON))
}