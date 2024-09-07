package main

import (
	"log"
	"os"
	"regexp"
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
	unique_domains := map[string]bool{}
	for _, line := range domains {
		domain := convert_to_domain_format(line)
		_, is_in_white_list := white_list[domain]
		if domain != "" && !is_in_white_list {
			unique_domains[domain] = true
		}
	}

	if skip_filter {
		return unique_domains
	}
	return filter_domain(unique_domains)
}

func filter_domain(domains map[string]bool) map[string]bool {
	splitted_domain_map := map[string]map[string]bool{}
	for k := range domains {
		splitted := strings.Split(k, ".")
		if len(splitted) < 2 {
			continue
		}
		domain_part := strings.Join(splitted[len(splitted)-2:], ".")
		if _, ok := splitted_domain_map[domain_part]; !ok {
			splitted_domain_map[domain_part] = map[string]bool{}
		}
		splitted_domain_map[domain_part][k] = true
	}
	filtered_domains := map[string]bool{}
	for k, v := range splitted_domain_map {
		if _, ok := v[k]; ok {
			filtered_domains[k] = true
		} else if _, ok := v["www."+k]; ok {
			filtered_domains[k] = true
		} else {
			for k2 := range v {
				filtered_domains[k2] = true
			}
		}
	}
	return filtered_domains
}
