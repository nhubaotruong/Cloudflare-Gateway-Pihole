package main

import (
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/imroc/req/v3"
	"golang.org/x/net/idna"
)

func read_domain_urls(file_name string) []string {
	// Open file
	content, err := os.ReadFile(file_name)
	if err != nil {
		log.Fatalln(err.Error())
	}
	lines := strings.Split(string(content), "\n")
	filtered_line := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		filtered_line = append(filtered_line, line)
	}

	// Read all lines
	var wg sync.WaitGroup
	var mu sync.Mutex
	result := []string{}
	client := req.NewClient()
	for _, line := range filtered_line {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			content := download_url(url, *client)
			mu.Lock()
			result = append(result, content...)
			mu.Unlock()
		}(line)
	}
	wg.Wait()

	return result
}

func download_url(url string, client req.Client) []string {
	resp := client.Get(url).Do()
	if resp.StatusCode != 200 || resp.Err != nil {
		log.Println("Error downloading", url, ". Status code", resp.StatusCode)
		return []string{}
	}
	body_text := resp.String()
	log.Println("Downloaded", url, ". File size", len(body_text))
	splitted_body := strings.Split(body_text, "\n")
	return splitted_body
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

func convert_to_domain_set(domains []string, skip_filter bool, white_list map[string]bool) map[string]bool {
	unique_domains := make(map[string]bool, len(domains))
	var mu sync.Mutex

	// Process domains in parallel
	workers := runtime.NumCPU()
	var wg sync.WaitGroup
	ch := make(chan string, len(domains))

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range ch {
				processed := convert_to_domain_format(domain)
				if processed != "" {
					mu.Lock()
					_, is_in_white_list := white_list[processed]
					if !is_in_white_list {
						unique_domains[processed] = true
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

func filter_domain(domains map[string]bool) map[string]bool {
	// Pre-allocate maps with estimated capacity
	splitted_domain_map := make(map[string]map[string]bool, len(domains))
	filtered_domains := make(map[string]bool, len(domains))

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
			splitted_domain_map[domain_part] = make(map[string]bool, 4) // Small initial capacity for subdomain map
		}
		splitted_domain_map[domain_part][domain] = true
	}

	// Filter domains
	var www_domain string
	for domain_part, subdomains := range splitted_domain_map {
		if subdomains[domain_part] {
			filtered_domains[domain_part] = true
			continue
		}
		www_domain = "www." + domain_part
		if subdomains[www_domain] {
			filtered_domains[domain_part] = true
			continue
		}
		for subdomain := range subdomains {
			filtered_domains[subdomain] = true
		}
	}

	return filtered_domains
}
