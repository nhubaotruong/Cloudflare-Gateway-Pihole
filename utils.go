package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/idna"
)

func read_domain_urls(file_name string) []string {
	var wg sync.WaitGroup
	var mu sync.Mutex
	// read file lists.txt into a list
	file, err := os.Open(file_name)
	if err != nil {
		fmt.Println(err.Error())
	}
	defer file.Close()
	// Read all lines
	scanner := bufio.NewScanner(file)
	result := [][]string{}
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "#") || text == "" {
			continue
		}
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			content := download_url(url)
			mu.Lock()
			result = append(result, content)
			mu.Unlock()
		}(text)
	}
	wg.Wait()

	// Return flatterned list
	flatterned := []string{}
	for _, v := range result {
		flatterned = append(flatterned, v...)
	}
	return flatterned
}

func download_url(url string) []string {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println(err.Error())
		return []string{}
	}
	defer resp.Body.Close()
	// Read response text
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return []string{}
	}
	body_text := string(body)
	fmt.Println("Downloaded", url, ". File size", len(body_text))
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
			fmt.Println("Error encoding domain:", err.Error())
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
	return filter_domain_using_tree(unique_domains)
}

func filter_domain_using_tree(domains map[string]bool) map[string]bool {
	rootNode := NewNode("")
	for domain := range domains {
		rootNode.AddDomain(domain)
	}
	path := make([]string, 0)
	result := make([][]string, 0)
	filtered_domains := make(map[string]bool)
	rootNode.Dfs(&path, &result)
	for _, branch := range result {
		_branch := branch[1:]
		reversed_branch := make([]string, len(_branch))
		for i := range _branch {
			reversed_branch[len(_branch)-1-i] = _branch[i]
		}
		filtered_domains[strings.Join(reversed_branch, ".")] = true
	}
	return filtered_domains
}
