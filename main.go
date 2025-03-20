package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var api_sleep_time = 4 * time.Second

func main() {
	for {
		if exec() == 0 {
			break
		}
		time.Sleep(api_sleep_time)
	}
}

func exec() int {
	defer func() int {
		if r := recover(); r != nil {
			return 1
		}
		return 0
	}()
	white_list_remote := read_domain_urls("whitelists.txt")
	white_list_static := read_file("whitelists_static.txt")
	white_list_set := convert_to_domain_set(append(white_list_remote, white_list_static...), true, nil)
	black_list := read_domain_urls("lists.txt")
	black_list_set := convert_to_domain_set(black_list, false, white_list_set)

	black_list_list := black_list_set.ToSortedList()

	log.Println("Total", len(black_list_list), "domains")

	// Write to file
	// os.WriteFile("block_list.txt", []byte(strings.Join(black_list_list, "\n")), 0644)
	// return 0

	// Get cf lists
	prefix := "[AdBlock-DNS Block List]"
	cf_lists := get_cf_lists(prefix)

	// Compare existing policies
	sum := 0
	for _, v := range cf_lists {
		sum += int(v.Count)
	}
	if len(black_list_list) == sum {
		log.Println("Lists are the same size, skipping")
		return 0
	}

	policy_prefix := fmt.Sprintf("%s Block Ads", prefix)
	deleted_policy := delete_gateway_policy(policy_prefix)
	log.Println("Deleted", deleted_policy, "gateway policy")

	// Delete cf lists
	for _, v := range cf_lists {
		log.Println("Deleting list", v.Name, "- ID:", v.ID)
		delete_cf_list(v.ID)
		time.Sleep(api_sleep_time)
	}

	// Create cf lists by 1000 chunks
	chunk_size := 1000
	chunk_counter := 0
	new_cf_lists := []cloudflare.TeamsList{}
	for i := 0; i < len(black_list_list); i += chunk_size {
		end := min(i+chunk_size, len(black_list_list))
		chunk_counter += 1
		name := fmt.Sprintf("%s %d", prefix, chunk_counter)
		log.Println("Creating list", name)
		cf_list := create_cf_list(name, black_list_list[i:end])
		new_cf_lists = append(new_cf_lists, cf_list)
		time.Sleep(api_sleep_time)
	}

	// Create cf policies
	new_cf_lists_ids := []string{}
	for _, v := range new_cf_lists {
		new_cf_lists_ids = append(new_cf_lists_ids, v.ID)
	}
	expected_cf_list_count := chunk_counter
	actual_cf_list_count := len(new_cf_lists_ids)
	if expected_cf_list_count != actual_cf_list_count {
		log.Println("Expected", expected_cf_list_count, "lists, but got", actual_cf_list_count)
		return 1
	}
	log.Println("Creating firewall policy")
	create_gateway_policy(policy_prefix, new_cf_lists_ids)
	log.Println("Done!")
	return 0
}
