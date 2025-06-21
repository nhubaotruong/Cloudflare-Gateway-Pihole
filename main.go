package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	for {
		if exec() == 0 {
			break
		}
		log.Println("Sleeping for 1 minute")
		time.Sleep(1 * time.Minute)
	}
}

func exec() int {
	white_list_remote := read_domain_urls("whitelists.txt")
	white_list_static := read_file("whitelists_static.txt")
	white_list_set := convert_to_domain_set(append(white_list_remote, white_list_static...), true, nil)
	black_list := read_domain_urls("lists.txt")
	black_list_set := convert_to_domain_set(black_list, false, white_list_set)
	// nrd_domains := get_nrd_domains()
	// for domain := range nrd_domains {
	// 	black_list_set.Add(domain)
	// }

	black_list_list := black_list_set.ToSortedList()

	log.Println("Total", len(black_list_list), "domains")

	if len(black_list_list) > 300000 {
		log.Println("black_list_list is longer than 300k, exiting with code 129")
		os.Exit(129)
	}

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
	var wg sync.WaitGroup
	for _, v := range cf_lists {
		wg.Add(1)
		go func(list zero_trust.GatewayList) {
			defer wg.Done()
			log.Println("Deleting list", list.Name, "- ID:", list.ID)
			delete_cf_list(list.ID)
		}(v)
	}
	wg.Wait()

	// Create cf lists by 1000 chunks
	chunk_size := 1000
	num_chunks := (len(black_list_list) + chunk_size - 1) / chunk_size
	resultsChan := make(chan zero_trust.GatewayList, num_chunks)
	var createWg sync.WaitGroup

	for i := 0; i < len(black_list_list); i += chunk_size {
		end := min(i+chunk_size, len(black_list_list))
		name := fmt.Sprintf("%s %d", prefix, (i/chunk_size)+1)
		createWg.Add(1)
		go func(name string, chunk []string) {
			defer createWg.Done()
			log.Println("Creating list", name)
			cf_list := create_cf_list(name, chunk)
			resultsChan <- cf_list
		}(name, black_list_list[i:end])
	}

	go func() {
		createWg.Wait()
		close(resultsChan)
	}()

	new_cf_lists := []zero_trust.GatewayList{}
	var creationFailed bool
	for result := range resultsChan {
		if result.ID == "" {
			creationFailed = true
		} else {
			new_cf_lists = append(new_cf_lists, result)
		}
	}

	if creationFailed {
		log.Println("One or more lists failed to create, retrying...")
		// Cleanup lists that were created successfully before retrying
		if len(new_cf_lists) > 0 {
			var deleteWg sync.WaitGroup
			for _, list := range new_cf_lists {
				deleteWg.Add(1)
				go func(listToDelete zero_trust.GatewayList) {
					defer deleteWg.Done()
					log.Println("Cleaning up created list", listToDelete.Name, "- ID:", listToDelete.ID)
					delete_cf_list(listToDelete.ID)
				}(list)
			}
			deleteWg.Wait()
		}
		return 1
	}

	// Create cf policies
	new_cf_lists_ids := []string{}
	for _, v := range new_cf_lists {
		new_cf_lists_ids = append(new_cf_lists_ids, v.ID)
	}
	log.Println("Creating firewall policy")
	create_gateway_policy(policy_prefix, new_cf_lists_ids)
	log.Println("Done!")
	return 0
}
