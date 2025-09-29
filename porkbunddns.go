package porkbunddns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	APIKey     string
	APISecret  string
	Domain     string
	Subdomains []string
	TTL        int
	IPv4File   string
	IPv6File   string
}

type PorkbunRequest struct {
	APIKey       string `json:"apikey"`
	SecretAPIKey string `json:"secretapikey"`
	Content      string `json:"content"`
	TTL          int    `json:"ttl"`
}

type PorkbunResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func getCurrentIPv4() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", fmt.Errorf("failed to get current IPv4. err=%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body. err=%w", err)
	}

	return strings.TrimSpace(string(body)), nil
}

func getCurrentIPv6() (string, error) {
	resp, err := http.Get("https://api64.ipify.org")
	if err != nil {
		return "", fmt.Errorf("failed to get current IPv6. err=%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body. err=%w", err)
	}

	return strings.TrimSpace(string(body)), nil
}

func getLastIP(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read IP file. err=%w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func saveIP(filename, ip string) error {
	if err := os.WriteFile(filename, []byte(ip), 0644); err != nil {
		return err
	}
	fmt.Println("wrote new ip to", filename)
	return nil
}

func updatePorkbunDNS(config Config, subdomain, ip, recordType string) error {
	url := fmt.Sprintf("https://api.porkbun.com/api/json/v3/dns/editByNameType/%s/%s/%s",
		config.Domain, recordType, subdomain)

	reqData := PorkbunRequest{
		APIKey:       config.APIKey,
		SecretAPIKey: config.APISecret,
		Content:      ip,
		TTL:          config.TTL,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data. err=%w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request. err=%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response. err=%w", err)
	}

	var porkbunResp PorkbunResponse
	if err := json.Unmarshal(body, &porkbunResp); err != nil {
		return fmt.Errorf("failed to parse response. err=%w", err)
	}

	if porkbunResp.Status != "SUCCESS" {
		return fmt.Errorf("API error. body=%s", string(body))
	}

	return nil
}

func UpdateDDNS(config Config) error {
	// check IPv4
	currentIPv4, err := getCurrentIPv4()
	if err != nil {
		return err
	}
	lastIPv4, err := getLastIP(config.IPv4File)
	if err != nil {
		return err
	}
	ipv4Changed := currentIPv4 != lastIPv4
	if !ipv4Changed {
		fmt.Printf("IPv4 has not changed: %s\n", currentIPv4)
	} else {
		fmt.Printf("IPv4 changed from %s to %s\n", lastIPv4, currentIPv4)
	}

	// check IPv6
	currentIPv6, err := getCurrentIPv6()
	if err != nil {
		return err
	}
	lastIPv6, err := getLastIP(config.IPv6File)
	if err != nil {
		return err
	}
	ipv6Changed := currentIPv6 != lastIPv6
	if !ipv6Changed {
		fmt.Printf("IPv6 has not changed: %s\n", currentIPv6)
	} else {
		fmt.Printf("IPv6 changed from %s to %s\n", lastIPv6, currentIPv6)
	}

	// Update DNS for each subdomain
	for _, subdomain := range config.Subdomains {
		displayName := subdomain
		if displayName == "" {
			displayName = config.Domain
		} else {
			displayName = subdomain + "." + config.Domain
		}
		if ipv4Changed {
			if err := updatePorkbunDNS(config, subdomain, currentIPv4, "A"); err != nil {
				return fmt.Errorf("error updating IPv4 DDNS for %s. err=%w", displayName, err)
			}
			fmt.Printf("A record updated: %s -> %s\n", displayName, currentIPv4)
		}

		if ipv6Changed {
			if err := updatePorkbunDNS(config, subdomain, currentIPv6, "AAAA"); err != nil {
				return fmt.Errorf("error updating IPv6 DDNS for %s. err=%w", displayName, err)
			}
			fmt.Printf("AAAA record updated: %s -> %s\n", displayName, currentIPv6)
		}
	}

	// Save IPs after all updates succeed
	if ipv4Changed {
		if err := saveIP(config.IPv4File, currentIPv4); err != nil {
			return err
		}
	}

	if ipv6Changed {
		if err := saveIP(config.IPv6File, currentIPv6); err != nil {
			return err
		}
	}
	return nil
}
