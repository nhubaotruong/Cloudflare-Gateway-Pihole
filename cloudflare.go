package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
)

var (
	account_id string
	cf_client  *cloudflare.Client
	ctx        context.Context
)

func init() {
	cf_api_token, has_cf_api_token := os.LookupEnv("CF_API_TOKEN")
	account_id_t, has_cf_identifier := os.LookupEnv("CF_IDENTIFIER")
	if !(has_cf_api_token && has_cf_identifier) && isRunningInGitHubActions() {
		log.Fatalln("Please set CF_API_TOKEN and CF_IDENTIFIER")
	}
	if cf_api_token != "" && account_id_t != "" {
		account_id = account_id_t
		// cf_identifier = cloudflare.AccountIdentifier(account_id)

		cf_client = cloudflare.NewClient(
			option.WithAPIToken(cf_api_token),
		)
	}
	ctx = context.Background()
}

func get_cf_lists(name_prefix string) []zero_trust.GatewayList {
	lists, err := cf_client.ZeroTrust.Gateway.Lists.List(ctx, zero_trust.GatewayListListParams{
		AccountID: cloudflare.F(account_id),
	})
	if err != nil {
		log.Println("Error response get_cf_lists", err.Error())
		return []zero_trust.GatewayList{}
	}
	filtered_lists := []zero_trust.GatewayList{}
	for _, v := range lists.Result {
		if strings.HasPrefix(v.Name, name_prefix) {
			filtered_lists = append(filtered_lists, v)
		}
	}
	return filtered_lists
}

func create_cf_list(name string, domains []string) zero_trust.GatewayList {
	items := []zero_trust.GatewayListNewParamsItem{}
	for _, d := range domains {
		items = append(items, zero_trust.GatewayListNewParamsItem{Value: cloudflare.F(d)})
	}
	req_params := zero_trust.GatewayListNewParams{
		AccountID:   cloudflare.F(account_id),
		Name:        cloudflare.F(name),
		Description: cloudflare.F("Created by script."),
		Type:        cloudflare.F(zero_trust.GatewayListNewParamsTypeDomain),
		Items:       cloudflare.F(items),
	}
	cf_list, err := cf_client.ZeroTrust.Gateway.Lists.New(ctx, req_params)
	if err != nil {
		log.Println("Error response create_cf_list", err.Error())
		return zero_trust.GatewayList{}
	}

	jsonBytes, err := json.Marshal(cf_list)
	if err != nil {
		log.Println("Error marshaling response:", err.Error())
		return zero_trust.GatewayList{}
	}

	var result zero_trust.GatewayList
	if err := result.UnmarshalJSON(jsonBytes); err != nil {
		log.Println("Error unmarshaling to GatewayList:", err.Error())
		return zero_trust.GatewayList{}
	}
	return result
}

func delete_cf_list(list_id string) {
	_, err := cf_client.ZeroTrust.Gateway.Lists.Delete(ctx, list_id, zero_trust.GatewayListDeleteParams{
		AccountID: cloudflare.F(account_id),
	})
	if err != nil {
		log.Println("Error response delete_cf_list", err.Error())
	}
}

func get_gateway_policies(name_prefix string) []zero_trust.GatewayRule {
	policies, err := cf_client.ZeroTrust.Gateway.Rules.List(ctx, zero_trust.GatewayRuleListParams{
		AccountID: cloudflare.F(account_id),
	})
	if err != nil {
		log.Println("Error response get_gateway_policies", err.Error())
		return []zero_trust.GatewayRule{}
	}
	filtered_policies := []zero_trust.GatewayRule{}
	for _, v := range policies.Result {
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
	teams_rule := zero_trust.GatewayRuleNewParams{
		AccountID:   cloudflare.F(account_id),
		Name:        cloudflare.F(name),
		Description: cloudflare.F("Created by script."),
		Action:      cloudflare.F(zero_trust.GatewayRuleNewParamsActionBlock),
		Enabled:     cloudflare.F(true),
		Filters:     cloudflare.F([]zero_trust.GatewayFilter{zero_trust.GatewayFilterDNS}),
		Traffic:     cloudflare.F(strings.Join(traffic, " or ")),
		RuleSettings: cloudflare.F(zero_trust.RuleSettingParam{
			BlockPageEnabled: cloudflare.F(false),
		}),
		Precedence: cloudflare.F(int64(5000)),
	}
	_, err := cf_client.ZeroTrust.Gateway.Rules.New(ctx, teams_rule)
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
	_, err := cf_client.ZeroTrust.Gateway.Rules.Delete(ctx, policy_id, zero_trust.GatewayRuleDeleteParams{
		AccountID: cloudflare.F(account_id),
	})
	if err != nil {
		log.Println("Error response delete_gateway_policy", err.Error())
		return 0
	}
	return 1
}
