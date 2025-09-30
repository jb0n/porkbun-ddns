package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	porkbunddns "github.com/jb0n/porkbun-ddns"
)

func main() {
	createFlag := flag.Bool("create", false, "Create configuration file interactively")
	flag.Parse()

	config, err := loadOrCreateConfig(*createFlag)
	if err != nil {
		log.Fatalf("Error loading configuration. err=%v", err)
	}

	if err := porkbunddns.UpdateDDNS(*config); err != nil {
		log.Fatalf("Error updating DDNS. err=%v", err)
	}
}

func loadOrCreateConfig(createMode bool) (*porkbunddns.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory. err=%w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "porkbun-ddns")
	configPath := filepath.Join(configDir, "config.json")

	if _, err := os.Stat(configPath); err == nil {
		if createMode {
			return nil, fmt.Errorf("configuration file already exists at %s", configPath)
		}
		return loadConfig(configPath)
	}

	if !createMode {
		return nil, fmt.Errorf("configuration file not found at %s. Run with --create flag to create it", configPath)
	}

	fmt.Println("Creating configuration file...")
	config := promptForConfig()

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory. err=%w", err)
	}
	if err := saveConfig(configPath, config); err != nil {
		return nil, err
	}

	fmt.Printf("Configuration saved to %s\n", configPath)
	return config, nil
}

func loadConfig(path string) (*porkbunddns.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file. err=%w", err)
	}
	config := &porkbunddns.Config{}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file. err=%w", err)
	}
	return config, nil
}

func saveConfig(path string, config *porkbunddns.Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config. err=%w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file. err=%w", err)
	}
	return nil
}

func promptForConfig() *porkbunddns.Config {
	reader := bufio.NewReader(os.Stdin)
	config := &porkbunddns.Config{
		APIKey:    promptString(reader, "Porkbun API Key", ""),
		APISecret: promptString(reader, "Porkbun API Secret", ""),
		Domain:    promptString(reader, "Domain", "example.com"),
		TTL:       promptInt(reader, "TTL (seconds)", "600"),
	}
	config.Subdomains = promptSubdomains(reader, config.Domain)
	return config
}

func promptString(reader *bufio.Reader, prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

func promptSubdomains(reader *bufio.Reader, domain string) []string {
	var subdomains []string

	updateBase := promptYesNo(reader, fmt.Sprintf("Update base domain record (%s)", domain))
	if updateBase {
		subdomains = append(subdomains, "")
	}

	subdomainInput := promptString(reader, "Subdomains to update (comma-separated)", "")
	if subdomainInput != "" {
		parts := strings.Split(subdomainInput, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				subdomains = append(subdomains, trimmed)
			}
		}
	}

	if len(subdomains) == 0 {
		fmt.Println("Error: Must update at least one record (base domain or subdomain).")
		return promptSubdomains(reader, domain)
	}

	return subdomains
}

func promptYesNo(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("%s (y/n) [y]: ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

func promptInt(reader *bufio.Reader, prompt, defaultValue string) int {
	input := promptString(reader, prompt, defaultValue)
	value, err := strconv.Atoi(input)
	if err != nil {
		log.Printf("Invalid integer, using default: %s", defaultValue)
		value, _ = strconv.Atoi(defaultValue)
	}
	return value
}
