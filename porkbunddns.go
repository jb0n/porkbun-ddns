package porkbunddns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type DNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     string `json:"ttl"`
	Prio    string `json:"prio"`
}

type RetrieveResponse struct {
	Status  string      `json:"status"`
	Records []DNSRecord `json:"records"`
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

func retrieveDNSRecords(config Config) ([]DNSRecord, error) {
	url := fmt.Sprintf("https://api.porkbun.com/api/json/v3/dns/retrieve/%s", config.Domain)

	reqData := struct {
		APIKey       string `json:"apikey"`
		SecretAPIKey string `json:"secretapikey"`
	}{
		APIKey:       config.APIKey,
		SecretAPIKey: config.APISecret,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data. err=%w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request. err=%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response. err=%w", err)
	}

	var retrieveResp RetrieveResponse
	if err := json.Unmarshal(body, &retrieveResp); err != nil {
		return nil, fmt.Errorf("failed to parse response. err=%w", err)
	}

	if retrieveResp.Status != "SUCCESS" {
		return nil, fmt.Errorf("API error. body=%s", string(body))
	}

	return retrieveResp.Records, nil
}

func findDNSRecord(records []DNSRecord, domain, subdomain, recordType string) *DNSRecord {
	expectedName := domain
	if subdomain != "" {
		expectedName = subdomain + "." + domain
	}

	for i := range records {
		if records[i].Type == recordType && records[i].Name == expectedName {
			return &records[i]
		}
	}
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
	// Get current public IPs
	currentIPv4, err := getCurrentIPv4()
	if err != nil {
		return err
	}
	currentIPv6, err := getCurrentIPv6()
	if err != nil {
		return err
	}

	// Retrieve existing DNS records from Porkbun API
	records, err := retrieveDNSRecords(config)
	if err != nil {
		return fmt.Errorf("failed to retrieve DNS records. err=%w", err)
	}

	// Check if any A or AAAA records need updating (only need to check one of each type)
	firstSubdomain := config.Subdomains[0]

	ipv4NeedsUpdate := false
	aRecord := findDNSRecord(records, config.Domain, firstSubdomain, "A")
	if aRecord != nil {
		if aRecord.Content != currentIPv4 {
			fmt.Printf("Current IPv4 is %s, DNS has %s (update needed)\n", currentIPv4, aRecord.Content)
			ipv4NeedsUpdate = true
		} else {
			fmt.Printf("IPv4 already up to date: %s\n", currentIPv4)
		}
	} else {
		fmt.Printf("A record not found (update needed)\n")
		ipv4NeedsUpdate = true
	}

	ipv6NeedsUpdate := false
	aaaaRecord := findDNSRecord(records, config.Domain, firstSubdomain, "AAAA")
	if aaaaRecord != nil {
		if aaaaRecord.Content != currentIPv6 {
			fmt.Printf("Current IPv6 is %s, DNS has %s (update needed)\n", currentIPv6, aaaaRecord.Content)
			ipv6NeedsUpdate = true
		} else {
			fmt.Printf("IPv6 already up to date: %s\n", currentIPv6)
		}
	} else {
		fmt.Printf("AAAA record not found (update needed)\n")
		ipv6NeedsUpdate = true
	}

	// Update DNS records only if needed
	if ipv4NeedsUpdate || ipv6NeedsUpdate {
		for _, subdomain := range config.Subdomains {
			displayName := subdomain
			if displayName == "" {
				displayName = config.Domain
			} else {
				displayName = subdomain + "." + config.Domain
			}

			if ipv4NeedsUpdate {
				if err := updatePorkbunDNS(config, subdomain, currentIPv4, "A"); err != nil {
					return fmt.Errorf("error updating IPv4 DDNS for %s. err=%w", displayName, err)
				}
				fmt.Printf("A record updated: %s -> %s\n", displayName, currentIPv4)
			}

			if ipv6NeedsUpdate {
				if err := updatePorkbunDNS(config, subdomain, currentIPv6, "AAAA"); err != nil {
					return fmt.Errorf("error updating IPv6 DDNS for %s. err=%w", displayName, err)
				}
				fmt.Printf("AAAA record updated: %s -> %s\n", displayName, currentIPv6)
			}
		}
	}
	return nil
}
