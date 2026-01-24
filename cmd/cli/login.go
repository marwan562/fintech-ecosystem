package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to the microservices platform",
	Run: func(cmd *cobra.Command, args []string) {
		var email, password string
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("Email: ")
		scanner.Scan()
		email = strings.TrimSpace(scanner.Text())
		fmt.Print("Password: ")
		scanner.Scan()
		password = strings.TrimSpace(scanner.Text())

		// Call Auth Service via Gateway
		loginReq := map[string]string{
			"email":    email,
			"password": password,
		}
		body, _ := json.Marshal(loginReq)

		gatewayURL := viper.GetString("gateway_url")
		if gatewayURL == "" {
			gatewayURL = "http://localhost:8080"
		}

		resp, err := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Printf("Error connecting to gateway: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Println("Login failed. Check your credentials.")
			return
		}

		var loginResp struct {
			Token string `json:"token"`
		}
		json.NewDecoder(resp.Body).Decode(&loginResp)

		// Get an API key for the user (test environment by default)
		client := &http.Client{}
		req, _ := http.NewRequest("POST", gatewayURL+"/auth/api_keys", bytes.NewBuffer([]byte(`{"environment":"test"}`)))
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		req.Header.Set("Content-Type", "application/json")

		respKey, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error generating API key: %v\n", err)
			return
		}
		defer respKey.Body.Close()

		if respKey.StatusCode != http.StatusCreated {
			fmt.Println("Failed to generate API key after login. Status:", respKey.Status)
			return
		}

		var keyResp struct {
			Key string `json:"key"`
		}
		json.NewDecoder(respKey.Body).Decode(&keyResp)

		// Save to config
		viper.Set("api_key", keyResp.Key)
		viper.Set("email", email)
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Warning: failed to write config: %v\n", err)
		}

		fmt.Println("Successfully logged in!")
		fmt.Printf("API Key stored: %s...%s\n", keyResp.Key[:7], keyResp.Key[len(keyResp.Key)-4:])
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
