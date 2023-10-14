package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/imroc/req/v3"
)

var (
	httpClient    *req.Client
	once          sync.Once
	cf_identifier string
)

func get_http_client() *req.Client {
	once.Do(func() {
		cf_api_token, has_cf_api_token := os.LookupEnv("CF_API_TOKEN")
		cf_identifier_1, has_cf_identifier := os.LookupEnv("CF_IDENTIFIER")
		if !(has_cf_api_token && has_cf_identifier) {
			log.Fatalln("Please set CF_API_TOKEN and CF_IDENTIFIER")
		}
		cf_identifier = cf_identifier_1
		httpClient = req.NewClient()
		httpClient.SetCommonBearerAuthToken(cf_api_token)
		httpClient.SetBaseURL("https://api.cloudflare.com")
		httpClient.SetCommonContentType("application/json")
		httpClient.SetCommonHeader("Accept", "application/json")
	})
	return httpClient
}

func get_cf_lists(name_prefix string) []interface{} {
	client := get_http_client()
	resp := client.Get(fmt.Sprintf("/client/v4/accounts/%s/gateway/lists", cf_identifier)).Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response get_cf_lists", resp.Err.Error(), "body", resp.String())
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
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
	request := client.Post(fmt.Sprintf("/client/v4/accounts/%s/gateway/lists", cf_identifier))
	request.SetBodyJsonMarshal(req_json)
	resp := request.Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response create_cf_list", resp.Err.Error(), "body", resp.String())
		return []interface{}{}
	}

	// Read body as marshalled json
	var result interface{}
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []interface{}{}
	}
	return result.(map[string]interface{})["result"]
}

func delete_cf_list(list_id string) interface{} {
	client := get_http_client()
	resp := client.Delete(fmt.Sprintf("/client/v4/accounts/%s/gateway/lists/%s", cf_identifier, list_id)).Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response delete_cf_list", resp.Err.Error(), "body", resp.String())
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []interface{}{}
	}
	return result.(map[string]interface{})["result"]
}

func get_gateway_policies(name_prefix string) []interface{} {
	client := get_http_client()
	resp := client.Get(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules", cf_identifier)).Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response get_gateway_policies", resp.Err.Error(), "body", resp.String())
		return []interface{}{}
	}

	// Read body as marshalled json
	var result interface{}
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []interface{}{}
	}
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
	request := client.Post(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules", cf_identifier))
	request.SetBodyJsonMarshal(req_json)
	resp := request.Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response create_gateway_policy", resp.Err.Error(), "body", resp.String())
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []interface{}{}
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
	request := client.Put(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules/%s", cf_identifier, policy_id))
	request.SetBodyJsonMarshal(req_json)
	resp := request.Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response update_gateway_policy", resp.Err.Error(), "body", resp.String())
		return []interface{}{}
	}
	// Read body as marshalled json
	var result interface{}
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []interface{}{}
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
	resp := client.Delete(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules/%s", cf_identifier, policy_id)).Do()
	if resp.Err != nil || resp.StatusCode != 200 {
		log.Fatalln("Error response delete_gateway_policy", resp.Err.Error(), "body", resp.String())
		return 0
	}
	return 1
}
