package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type headerRoundTripper struct {
	headers http.Header
	rt      http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header[k] = v
	}
	return h.rt.RoundTrip(req)
}

var (
	httpClient *http.Client
	once       sync.Once
)

func get_http_client() *http.Client {
	once.Do(func() {
		httpClient = &http.Client{
			Transport: &headerRoundTripper{
				headers: http.Header{
					"Authorization": {"Bearer " + os.Getenv("CF_API_TOKEN")},
				},
				rt: http.DefaultTransport,
			},
		}
	})
	return httpClient
}

func get_cf_lists(name_prefix string) []interface{} {
	client := get_http_client()
	resp, err := client.Get(fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/gateway/lists", os.Getenv("CF_IDENTIFIER")))
	if err != nil {
		fmt.Println(err.Error())
		return []interface{}{}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return []interface{}{}
	}
	if resp.StatusCode != 200 {
		fmt.Println(string(body))
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println(err.Error())
		return []interface{}{}
	}
	// Get lists
	lists := result.(map[string]interface{})["result"].([]interface{})
	// Filter lists
	filtered_lists := []interface{}{}
	for _, v := range lists {
		list_name := v.(map[string]interface{})["name"].(string)
		if strings.HasPrefix(list_name, name_prefix) {
			filtered_lists = append(filtered_lists, v)
		}
	}
	return filtered_lists
}

func create_cf_list(name string, domains []string) interface{} {
	client := get_http_client()
	items := []map[string]string{}
	for _, d := range domains {
		items = append(items, map[string]string{"value": d})
	}
	req_json := map[string]interface{}{
		"name":        name,
		"description": "Created by script.",
		"type":        "DOMAIN",
		"items":       items,
	}
	req_json_bytes, err := json.Marshal(req_json)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	body_io_reader := strings.NewReader(string(req_json_bytes))
	resp, err := client.Post(fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/gateway/lists", os.Getenv("CF_IDENTIFIER")), "application/json", body_io_reader)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	if resp.StatusCode != 200 {
		fmt.Println(string(body))
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	return result.(map[string]interface{})["result"]
}

func delete_cf_list(list_id string) interface{} {
	client := get_http_client()
	resp, err := client.Do(&http.Request{
		Method: "DELETE",
		URL:    &url.URL{Scheme: "https", Host: "api.cloudflare.com", Path: fmt.Sprintf("/client/v4/accounts/%s/gateway/lists/%s", os.Getenv("CF_IDENTIFIER"), list_id)},
	})
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	if resp.StatusCode != 200 {
		fmt.Println(string(body))
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	return result.(map[string]interface{})["result"]
}

func get_gateway_policies(name_prefix string) []interface{} {
	client := get_http_client()
	resp, err := client.Get(fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/gateway/rules", os.Getenv("CF_IDENTIFIER")))
	if err != nil {
		fmt.Println(err.Error())
		return []interface{}{}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return []interface{}{}
	}
	if resp.StatusCode != 200 {
		fmt.Println(string(body))
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println(err.Error())
		return []interface{}{}
	}
	// Get lists
	lists := result.(map[string]interface{})["result"].([]interface{})
	// Filter lists
	filtered_lists := []interface{}{}
	for _, v := range lists {
		list_name := v.(map[string]interface{})["name"].(string)
		if strings.HasPrefix(list_name, name_prefix) {
			filtered_lists = append(filtered_lists, v)
		}
	}
	return filtered_lists
}

func create_gateway_policy(name string, list_ids []string) interface{} {
	client := get_http_client()
	traffic := []string{}
	for _, l := range list_ids {
		traffic = append(traffic, fmt.Sprintf("any(dns.domains[*] in $%s)", l))
	}
	req_json := map[string]interface{}{
		"name":        name,
		"description": "Created by script.",
		"action":      "block",
		"enabled":     true,
		"filters":     []string{"dns"},
		"traffic":     strings.Join(traffic, " or "),
		"rule_settings": map[string]interface{}{
			"block_page_enabled": false,
		},
	}
	req_json_bytes, err := json.Marshal(req_json)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	body_io_reader := strings.NewReader(string(req_json_bytes))
	resp, err := client.Post(fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/gateway/rules", os.Getenv("CF_IDENTIFIER")), "application/json", body_io_reader)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	if resp.StatusCode != 200 {
		fmt.Println(string(body))
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	return result.(map[string]interface{})["result"]
}

func update_gateway_policy(name string, policy_id string, list_ids []string) interface{} {
	client := get_http_client()
	traffic := []string{}
	for _, l := range list_ids {
		traffic = append(traffic, fmt.Sprintf("any(dns.domains[*] in $%s)", l))
	}
	req_json := map[string]interface{}{
		"name":    name,
		"action":  "block",
		"enabled": true,
		"traffic": strings.Join(traffic, " or "),
	}
	req_json_bytes, err := json.Marshal(req_json)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	body_io_reader := strings.NewReader(string(req_json_bytes))
	resp, err := client.Do(&http.Request{
		Method: "PUT",
		URL:    &url.URL{Scheme: "https", Host: "api.cloudflare.com", Path: fmt.Sprintf("/client/v4/accounts/%s/gateway/rules/%s", os.Getenv("CF_IDENTIFIER"), policy_id)},
		Body:   io.NopCloser(body_io_reader),
	})
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	if resp.StatusCode != 200 {
		fmt.Println(string(body))
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	return result.(map[string]interface{})["result"]
}

func delete_gateway_policy(policy_name_prefix string) int {
	client := get_http_client()
	policies := get_gateway_policies(policy_name_prefix)
	if len(policies) == 0 {
		return 0
	}
	policy_id := policies[0].(map[string]interface{})["id"].(string)
	resp, err := client.Do(&http.Request{
		Method: "DELETE",
		URL:    &url.URL{Scheme: "https", Host: "api.cloudflare.com", Path: fmt.Sprintf("/client/v4/accounts/%s/gateway/rules/%s", os.Getenv("CF_IDENTIFIER"), policy_id)},
	})
	if err != nil {
		fmt.Println(err.Error())
		return 0
	}
	defer resp.Body.Close()
	return 1
}
