package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"github.com/imroc/req/v3"
)

var (
	cf_identifier string
	http_client   *req.Client
)

func init() {
	cf_api_token, has_cf_api_token := os.LookupEnv("CF_API_TOKEN")
	cf_identifier_1, has_cf_identifier := os.LookupEnv("CF_IDENTIFIER")
	if !(has_cf_api_token && has_cf_identifier) {
		log.Fatalln("Please set CF_API_TOKEN and CF_IDENTIFIER")
	}
	cf_identifier = cf_identifier_1
	http_client = req.NewClient()
	http_client.SetCommonBearerAuthToken(cf_api_token)
	http_client.SetBaseURL("https://api.cloudflare.com")
	http_client.SetCommonContentType("application/json")
	http_client.SetCommonHeader("Accept", "application/json")
}

func get_cf_lists(name_prefix string) []cloudflare.TeamsList {
	resp := http_client.Get(fmt.Sprintf("/client/v4/accounts/%s/gateway/lists", cf_identifier)).Do()
	if resp.Err != nil {
		log.Fatalln("Error response get_cf_lists", resp.Err.Error(), "body", resp.String())
	} else if resp.StatusCode != 200 {
		log.Fatalln("Error response get_cf_lists", resp.StatusCode, "body", resp.String())
	}
	// Read body as marshalled json
	var result cloudflare.TeamsListListResponse
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []cloudflare.TeamsList{}
	}
	// Get lists
	lists := result.Result
	// Filter lists
	filtered_lists := []cloudflare.TeamsList{}
	for _, v := range lists {
		if strings.HasPrefix(v.Name, name_prefix) {
			filtered_lists = append(filtered_lists, v)
		}
	}
	return filtered_lists
}

func create_cf_list(name string, domains []string) cloudflare.TeamsList {
	items := []cloudflare.TeamsListItem{}
	for _, d := range domains {
		items = append(items, cloudflare.TeamsListItem{Value: d})
	}
	req_json := cloudflare.TeamsList{
		Name:        name,
		Description: "Created by script.",
		Type:        "DOMAIN",
		Items:       items,
	}
	request := http_client.Post(fmt.Sprintf("/client/v4/accounts/%s/gateway/lists", cf_identifier))
	request.SetBodyJsonMarshal(req_json)
	resp := request.Do()
	if resp.Err != nil {
		log.Fatalln("Error response create_cf_list", resp.Err.Error(), "body", resp.String())
	} else if resp.StatusCode != 200 {
		log.Fatalln("Error response create_cf_list", resp.StatusCode, "body", resp.String())
	}

	var result cloudflare.TeamsListDetailResponse
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return cloudflare.TeamsList{}
	}
	return result.Result
}

func delete_cf_list(list_id string) {
	resp := http_client.Delete(fmt.Sprintf("/client/v4/accounts/%s/gateway/lists/%s", cf_identifier, list_id)).Do()
	if resp.Err != nil {
		log.Fatalln("Error response delete_cf_list", resp.Err.Error(), "body", resp.String())
	} else if resp.StatusCode != 200 {
		log.Fatalln("Error response delete_cf_list", resp.StatusCode, "body", resp.String())
	}
}

func get_gateway_policies(name_prefix string) []cloudflare.TeamsRule {
	resp := http_client.Get(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules", cf_identifier)).Do()
	if resp.Err != nil {
		log.Fatalln("Error response get_gateway_policies", resp.Err.Error(), "body", resp.String())
	}
	var result cloudflare.TeamsRulesResponse
	err := resp.UnmarshalJson(&result)
	if err != nil {
		log.Fatalln("Error unmarshalling json", err.Error())
		return []cloudflare.TeamsRule{}
	}
	policies := result.Result
	filtered_policies := []cloudflare.TeamsRule{}
	for _, v := range policies {
		if strings.HasPrefix(v.Name, name_prefix) {
			filtered_policies = append(filtered_policies, v)
		}
	}
	return filtered_policies

}

func create_gateway_policy(name string, list_ids []string) {
	traffic := []string{}
	for _, l := range list_ids {
		traffic = append(traffic, fmt.Sprintf("any(dns.domains[*] in $%s)", l))
	}
	teams_rule := cloudflare.TeamsRule{
		Name:        name,
		Description: "Created by script.",
		Action:      cloudflare.Block,
		Enabled:     true,
		Filters:     []cloudflare.TeamsFilterType{cloudflare.DnsFilter},
		Traffic:     strings.Join(traffic, " or "),
		RuleSettings: cloudflare.TeamsRuleSettings{
			BlockPageEnabled: false,
		},
	}
	req := http_client.Post(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules", cf_identifier))
	req.SetBodyJsonMarshal(teams_rule)
	resp := req.Do()
	if resp.Err != nil {
		log.Fatalln("Error response create_gateway_policy", resp.Err.Error(), "body", resp.String())
	} else if resp.StatusCode != 200 {
		log.Fatalln("Error response create_gateway_policy", resp.StatusCode, "body", resp.String())
	}
	// cf_client.TeamsCreateRule(ctx, cf_identifier, teams_rule)
}

func delete_gateway_policy(policy_name_prefix string) int {
	policies := get_gateway_policies(policy_name_prefix)
	if len(policies) == 0 {
		return 0
	}
	policy_id := policies[0].ID
	resp := http_client.Delete(fmt.Sprintf("/client/v4/accounts/%s/gateway/rules/%s", cf_identifier, policy_id)).Do()
	if resp.Err != nil {
		log.Fatalln("Error response delete_gateway_policy", resp.Err.Error(), "body", resp.String())
		return 0
	} else if resp.StatusCode != 200 {
		log.Fatalln("Error response delete_gateway_policy", resp.StatusCode, "body", resp.String())
		return 0
	}
	return 1
}
