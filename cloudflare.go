package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudflare/cloudflare-go"
)

var (
	account_id    string
	cf_client     *cloudflare.API
	cf_identifier *cloudflare.ResourceContainer
	ctx           context.Context
)

func init() {
	cf_api_token, has_cf_api_token := os.LookupEnv("CF_API_TOKEN")
	account_id_t, has_cf_identifier := os.LookupEnv("CF_IDENTIFIER")
	if !(has_cf_api_token && has_cf_identifier) && isRunningInGitHubActions() {
		log.Fatalln("Please set CF_API_TOKEN and CF_IDENTIFIER")
	}
	if cf_api_token != "" && account_id_t != "" {
		account_id = account_id_t
		cf_identifier = cloudflare.AccountIdentifier(account_id)

		var err error
		cf_client, err = cloudflare.NewWithAPIToken(cf_api_token)
		if err != nil {
			log.Fatalln("Error creating Cloudflare client:", err)
		}
		ctx = context.Background()
	}
}

func get_cf_lists(name_prefix string) []cloudflare.TeamsList {
	lists, _, err := cf_client.ListTeamsLists(ctx, cf_identifier, cloudflare.ListTeamListsParams{})
	if err != nil {
		log.Println("Error response get_cf_lists", err.Error())
		return []cloudflare.TeamsList{}
	}
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
	req_json := cloudflare.CreateTeamsListParams{
		Name:        name,
		Description: "Created by script.",
		Type:        "DOMAIN",
		Items:       items,
	}
	cf_list, err := cf_client.CreateTeamsList(ctx, cf_identifier, req_json)
	if err != nil {
		log.Println("Error response create_cf_list", err.Error())
		return cloudflare.TeamsList{}
	}
	return cf_list
}

func delete_cf_list(list_id string) {
	err := cf_client.DeleteTeamsList(ctx, cf_identifier, list_id)
	if err != nil {
		log.Println("Error response delete_cf_list", err.Error())
	}
}

func get_gateway_policies(name_prefix string) []cloudflare.TeamsRule {
	policies, err := cf_client.TeamsRules(ctx, account_id)
	if err != nil {
		log.Println("Error response get_gateway_policies", err.Error())
		return []cloudflare.TeamsRule{}
	}
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
		Precedence: 5000,
	}
	_, err := cf_client.TeamsCreateRule(ctx, account_id, teams_rule)
	if err != nil {
		log.Println("Error response create_gateway_policy", err.Error())
	}
}

func delete_gateway_policy(policy_name_prefix string) int {
	policies := get_gateway_policies(policy_name_prefix)
	if len(policies) == 0 {
		return 0
	}
	policy_id := policies[0].ID
	err := cf_client.TeamsDeleteRule(ctx, account_id, policy_id)
	if err != nil {
		log.Println("Error response delete_gateway_policy", err.Error())
		return 0
	}
	return 1
}
