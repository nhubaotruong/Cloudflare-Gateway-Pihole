package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
)

var httpClient *http.Client

func init() {
	// Configure optimized HTTP client
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         dialer.DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: runtime.NumCPU() + 1, // Match number of CPUs + 1
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

func isRunningInGitHubActions() bool {
	// GitHub Actions sets GITHUB_ACTIONS=true
	return os.Getenv("GITHUB_ACTIONS") == "true"
}

// retryableHTTPGet performs HTTP GET with retries
func retryableHTTPGet(req *http.Request) (*http.Response, error) {
	maxRetries := 3
	backoff := 1 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(backoff * time.Duration(i))
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Only retry on 5xx server errors
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("after %d retries, last error: %v", maxRetries, lastErr)
}

func read_file(file_name string) []string {
	// Open file
	content, err := os.ReadFile(file_name)
	if err != nil {
		log.Fatalln(err.Error())
	}
	lines := strings.Split(string(content), "\n")
	filtered_line := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		filtered_line = append(filtered_line, line)
	}
	return filtered_line
}

func read_domain_urls(file_name string) []string {
	// Open file
	filtered_line := read_file(file_name)

	// Initialize channels for work distribution
	numWorkers := runtime.NumCPU() * 2 // Use 2x CPU cores for I/O bound tasks
	urls := make(chan string, len(filtered_line))
	results := make(chan []string, len(filtered_line))

	// Start worker pool
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urls {
				results <- download_url(url)
			}
		}()
	}

	// Send work to workers
	go func() {
		for _, url := range filtered_line {
			urls <- url
		}
		close(urls)
	}()

	// Wait for all workers to finish in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var finalResult []string
	for domains := range results {
		finalResult = append(finalResult, domains...)
	}

	return finalResult
}

func download_url(url string) []string {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Error creating request for %s: %v\n", url, err)
		return []string{}
	}

	// Add common headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/plain,text/html")

	resp, err := retryableHTTPGet(req)
	if err != nil {
		log.Printf("Error downloading %s: %v\n", url, err)
		return []string{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error downloading %s. Status code: %d\n", url, resp.StatusCode)
		return []string{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body from %s: %v\n", url, err)
		return []string{}
	}

	body_text := string(body)
	log.Printf("Downloaded %s. File size: %d\n", url, len(body_text))
	return strings.Split(body_text, "\n")
}

var block_nrd_pattern = regexp.MustCompile(`(?i)^[\w\d-]+(bet|casino)\d*\.[\w]{2,}$`)

func get_nrd_domains() DomainSet {
	raw_domains := download_url("https://raw.githubusercontent.com/xRuffKez/NRD/refs/heads/main/lists/30-day-mini/domains-only/nrd-30day-mini.txt")
	filtered_domains := []string{}
	for _, domain := range raw_domains {
		if block_nrd_pattern.MatchString(domain) {
			log.Println("NRD:", domain)
			filtered_domains = append(filtered_domains, domain)
		}
	}
	return convert_to_domain_set(filtered_domains, false, DomainSet{})
}

var replace_pattern = regexp.MustCompile(`(^([0-9.]+|[0-9a-fA-F:.]+)\s+|^(\|\||@@\|\|?|\*\.|\*))`)
var domain_pattern = regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))*$`)
var ip_pattern = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)

func convert_to_domain_format(domain string) string {
	linex := strings.TrimSpace(domain)
	if strings.HasPrefix(linex, "#") || strings.HasPrefix(linex, "!") || strings.HasPrefix(linex, "/") || linex == "" {
		return ""
	}
	// convert to domains
	linex = strings.ToLower(linex)
	linex = strings.Split(linex, "#")[0]
	linex = strings.Split(linex, "^")[0]
	linex = strings.Split(linex, "$")[0]
	linex = strings.ReplaceAll(linex, "\r", "")
	linex = strings.TrimSpace(linex)
	linex = strings.TrimPrefix(linex, "*.")
	linex = strings.TrimPrefix(linex, ".")
	linex = replace_pattern.ReplaceAllString(linex, "")
	// Convert idna
	domain_x, err := idna.ToASCII(linex)
	if err != nil {
		log.Println("Error encoding domain:", err.Error())
		return ""
	}

	// Check if domain
	if !domain_pattern.MatchString(domain_x) || ip_pattern.MatchString(domain_x) {
		return ""
	}
	return strings.TrimPrefix(domain_x, "www.")
}

func convert_to_domain_set(domains []string, skip_filter bool, white_list DomainSet) DomainSet {
	unique_domains := make(DomainSet, len(domains))
	var mu sync.Mutex

	// Process domains in parallel
	workers := runtime.NumCPU()
	var wg sync.WaitGroup
	ch := make(chan string, len(domains))

	// Start workers
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range ch {
				processed := convert_to_domain_format(domain)
				if processed != "" {
					mu.Lock()
					if !white_list.Contains(processed) {
						unique_domains.Add(processed)
					}
					mu.Unlock()
				}
			}
		}()
	}

	// Feed domains to workers
	for _, domain := range domains {
		ch <- domain
	}
	close(ch)
	wg.Wait()

	if skip_filter {
		return unique_domains
	}
	return filter_domain(unique_domains)
}

func filter_domain(domains DomainSet) DomainSet {
	// Pre-allocate maps with estimated capacity
	splitted_domain_map := make(map[string]DomainSet, len(domains))
	filtered_domains := make(DomainSet, len(domains))

	// Process domains
	var parts []string
	for domain := range domains {
		parts = strings.SplitN(domain, ".", -1)
		if len(parts) < 2 {
			continue
		}
		// Get last two parts efficiently
		domain_part := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if _, ok := splitted_domain_map[domain_part]; !ok {
			splitted_domain_map[domain_part] = make(DomainSet) // Small initial capacity for subdomain map
		}
		splitted_domain_map[domain_part].Add(domain)
	}

	// Filter domains
	var www_domain string
	for domain_part, subdomains := range splitted_domain_map {
		if subdomains.Contains(domain_part) {
			filtered_domains.Add(domain_part)
			continue
		}
		www_domain = "www." + domain_part
		if subdomains.Contains(www_domain) {
			filtered_domains.Add(domain_part)
			continue
		}
		for subdomain := range subdomains {
			filtered_domains.Add(subdomain)
		}
	}

	return filtered_domains
}
