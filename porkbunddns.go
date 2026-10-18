package porkbunddns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

type Config struct {
	APIKey     string
	APISecret  string
	Domain     string
	Subdomains []string
	TTL        int
}

type PorkbunRequest struct {
	APIKey       string `json:"apikey"`
	SecretAPIKey string `json:"secretapikey"`
	Content      string `json:"content"`
	TTL          int    `json:"ttl"`
}

type PorkbunCreateRequest struct {
	APIKey       string `json:"apikey"`
	SecretAPIKey string `json:"secretapikey"`
	Name         string `json:"name"`
	Type         string `json:"type"`
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

func fetchPublicIP(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body. err=%w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from %s: %s", url, resp.Status)
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP address from %s: %q", url, ip)
	}
	return ip, nil
}

func getCurrentIPv4() (string, error) {
	ip, err := fetchPublicIP("https://api.ipify.org")
	if err != nil {
		return "", fmt.Errorf("failed to get current IPv4. err=%w", err)
	}
	if strings.Contains(ip, ":") {
		return "", fmt.Errorf("expected an IPv4 address from api.ipify.org, got %s", ip)
	}
	return ip, nil
}

// getCurrentIPv6 returns an empty string when the host has no public IPv6
// connectivity. A dual-stack probe such as api64.ipify.org would return the
// IPv4 address in that case, which must not be written to an AAAA record.
func getCurrentIPv6() string {
	ip, err := fetchPublicIP("https://api6.ipify.org")
	if err != nil {
		return ""
	}
	if net.ParseIP(ip).To4() != nil {
		return ""
	}
	return ip
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

	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
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

	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
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

func createPorkbunDNS(config Config, subdomain, ip, recordType string) error {
	url := fmt.Sprintf("https://api.porkbun.com/api/json/v3/dns/create/%s", config.Domain)

	reqData := PorkbunCreateRequest{
		APIKey:       config.APIKey,
		SecretAPIKey: config.APISecret,
		Name:         subdomain,
		Type:         recordType,
		Content:      ip,
		TTL:          config.TTL,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data. err=%w", err)
	}

	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
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
	currentIPv6 := getCurrentIPv6()
	if currentIPv6 == "" {
		fmt.Println("No public IPv6; skipping AAAA records")
	}

	// Retrieve existing DNS records from Porkbun API
	records, err := retrieveDNSRecords(config)
	if err != nil {
		return fmt.Errorf("failed to retrieve DNS records. err=%w", err)
	}

	var updateErrs []error

	for _, subdomain := range config.Subdomains {
		displayName := subdomain
		if displayName == "" {
			displayName = config.Domain
		} else {
			displayName = subdomain + "." + config.Domain
		}

		aRecord := findDNSRecord(records, config.Domain, subdomain, "A")
		switch {
		case aRecord == nil:
			if err := createPorkbunDNS(config, subdomain, currentIPv4, "A"); err != nil {
				updateErrs = append(updateErrs, fmt.Errorf("error creating IPv4 DDNS for %s. err=%w", displayName, err))
			} else {
				fmt.Printf("A record created: %s -> %s\n", displayName, currentIPv4)
			}
		case aRecord.Content != currentIPv4:
			if err := updatePorkbunDNS(config, subdomain, currentIPv4, "A"); err != nil {
				updateErrs = append(updateErrs, fmt.Errorf("error updating IPv4 DDNS for %s. err=%w", displayName, err))
			} else {
				fmt.Printf("A record updated: %s -> %s\n", displayName, currentIPv4)
			}
		default:
			fmt.Printf("A already up to date for %s: %s\n", displayName, currentIPv4)
		}

		if currentIPv6 == "" {
			continue
		}

		aaaaRecord := findDNSRecord(records, config.Domain, subdomain, "AAAA")
		switch {
		case aaaaRecord == nil:
			if err := createPorkbunDNS(config, subdomain, currentIPv6, "AAAA"); err != nil {
				updateErrs = append(updateErrs, fmt.Errorf("error creating IPv6 DDNS for %s. err=%w", displayName, err))
			} else {
				fmt.Printf("AAAA record created: %s -> %s\n", displayName, currentIPv6)
			}
		case aaaaRecord.Content != currentIPv6:
			if err := updatePorkbunDNS(config, subdomain, currentIPv6, "AAAA"); err != nil {
				updateErrs = append(updateErrs, fmt.Errorf("error updating IPv6 DDNS for %s. err=%w", displayName, err))
			} else {
				fmt.Printf("AAAA record updated: %s -> %s\n", displayName, currentIPv6)
			}
		default:
			fmt.Printf("AAAA already up to date for %s: %s\n", displayName, currentIPv6)
		}
	}

	return errors.Join(updateErrs...)
}
