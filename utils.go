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
	result := make([][]string, len(filtered_line))
	client := req.NewClient()
	for i, line := range filtered_line {
		wg.Add(1)
		go func(url string, i int) {
			defer wg.Done()
			content := download_url(url, *client)
			result[i] = content
		}(line, i)
	}
	wg.Wait()

	// Return flatterned list
	flatterned := []string{}
	for _, v := range result {
		flatterned = append(flatterned, v...)
	}
	return flatterned
}

func download_url(url string, client req.Client) []string {
	resp := client.Get(url).Do()
	if resp.StatusCode != 200 || resp.Err != nil {
		log.Fatalln("Error downloading", url, ". Status code", resp.StatusCode)
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

func convert_to_domain_set(domains []string, skip_filter bool) map[string]bool {
	unique_domains := make(map[string]bool)
	for _, line := range domains {
		// skip comments and empty lines
		linex := strings.TrimSpace(line)
		if strings.HasPrefix(linex, "#") || strings.HasPrefix(linex, "!") || strings.HasPrefix(linex, "/") || linex == "" {
			continue
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
		domain, err := idna.ToASCII(linex)
		if err != nil {
			log.Println("Error encoding domain:", err.Error())
			continue
		}

		// remove not domains
		if !domain_pattern.MatchString(domain) || ip_pattern.MatchString(domain) {
			continue
		}
		unique_domains[domain] = true
	}
	if skip_filter {
		return unique_domains
	}
	return filter_domain(unique_domains)
}

func filter_domain(domains map[string]bool) map[string]bool {
	splitted_domain_map := make(map[string]map[string]bool)
	for k := range domains {
		splitted := strings.Split(k, ".")
		if len(splitted) < 2 {
			continue
		}
		domain_part := strings.Join(splitted[len(splitted)-2:], ".")
		if _, ok := splitted_domain_map[domain_part]; !ok {
			splitted_domain_map[domain_part] = make(map[string]bool)
		}
		splitted_domain_map[domain_part][k] = true
	}
	filtered_domains := make(map[string]bool)
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
