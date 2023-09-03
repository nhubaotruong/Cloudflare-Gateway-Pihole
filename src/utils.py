import logging
import re

import requests
from requests.adapters import HTTPAdapter

from src import cloudflare

simple_ip_regex = re.compile(r"^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\s*(.+)$")

valid_domain_regex = re.compile(r"^((?!-)[A-Za-z0-9-]{1,63}(?<!-)\.)+[A-Za-z]{2,6}")


# Create session with 3 retries
list_download_session = requests.Session()
list_download_session.mount("http://", HTTPAdapter(max_retries=3))
list_download_session.mount("https://", HTTPAdapter(max_retries=3))


class App:
    def __init__(
        self, adlist_name: str, adlist_urls: list[str], whitelist_urls: list[str]
    ):
        self.adlist_name = adlist_name
        self.adlist_urls = adlist_urls
        self.whitelist_urls = whitelist_urls
        self.name_prefix = f"[AdBlock-{adlist_name}]"

    def run(self):
        file_content = ""
        whitelist_content = ""
        for url in self.adlist_urls:
            file_content += self.download_file(url)
        for url in self.whitelist_urls:
            whitelist_content += self.download_file(url)
        unfiltered_domains = self.convert_to_domain_list(file_content)
        whitelist_domains = self.convert_to_domain_list(whitelist_content)

        # remove whitelisted domains
        domains = list(set(unfiltered_domains) - set(whitelist_domains))
        logging.info(f"Number of domains after filtering: {len(domains)}")

        # check if the list is already in Cloudflare
        cf_lists = cloudflare.get_lists(self.name_prefix)

        logging.info(f"Number of lists in Cloudflare: {len(cf_lists)}")

        # compare the lists size
        if len(domains) == sum([l["count"] for l in cf_lists]):
            logging.warning("Lists are the same size, skipping")
            return

        # Delete existing policy created by script
        policy_prefix = f"{self.name_prefix} Block Ads"
        deleted_policies = cloudflare.delete_gateway_policy(policy_prefix)
        logging.info(f"Deleted {deleted_policies} gateway policies")

        # delete the lists
        for l in cf_lists:
            logging.info(f"Deleting list {l['name']} - ID:{l['id']} ")
            cloudflare.delete_list(l["name"], l["id"])

        cf_lists = []

        # chunk the domains into lists of 1000 and create them
        for chunk in self.chunk_list(domains, 1000):
            list_name = f"{self.name_prefix} {len(cf_lists) + 1}"
            logging.info(f"Creating list {list_name}")
            _list = cloudflare.create_list(list_name, chunk)
            if _list:
                cf_lists.append(_list)

        # get the gateway policies
        cf_policies = cloudflare.get_firewall_policies(self.name_prefix)

        logging.info(f"Number of policies in Cloudflare: {len(cf_policies)}")

        # setup the gateway policy
        if len(cf_policies) == 0:
            logging.info("Creating firewall policy")
            cf_policies = cloudflare.create_gateway_policy(
                f"{self.name_prefix} Block Ads", [l["id"] for l in cf_lists]
            )

        elif len(cf_policies) != 1:
            logging.error("More than one firewall policy found")
            raise Exception("More than one firewall policy found")

        else:
            logging.info("Updating firewall policy")
            cloudflare.update_gateway_policy(
                f"{self.name_prefix} Block Ads",
                cf_policies[0]["id"],
                [l["id"] for l in cf_lists],
            )

        logging.info("Done")

    def download_file(self, url: str):
        logging.info(f"Downloading file from {url}")
        r = list_download_session.get(url, allow_redirects=True)
        text = r.content.decode("utf-8")
        # Workaround for stevenblack
        if "# Start StevenBlack" in text:
            text = text.split("# Start StevenBlack")[1]
        logging.info(f"File size: {len(r.content)}")
        return text

    def convert_to_domain_list(self, file_content: str):
        skip_domains = [
            "localhost",
            "local",
            "localhost.localdomain",
        ]

        domains = []

        for _line in file_content.splitlines():
            # skip comments and empty lines
            line = _line.strip()
            if line.startswith("#") or line == "":
                continue

            if domain_search := simple_ip_regex.search(line):
                domain = domain_search.group(1).strip().lower()
            else:
                domain = line.strip().lower()

            if "#" in domain:
                domain = domain.split("#")[0].strip().lower()

            if domain in skip_domains:
                continue

            if not bool(valid_domain_regex.match(domain)):
                continue

            domains.append(domain)

        domains = sorted(list(set(domains)))

        logging.info(f"Number of domains: {len(domains)}")

        return domains

    def chunk_list(self, _list: list[str], n: int):
        for i in range(0, len(_list), n):
            yield _list[i : i + n]

    def delete(self):
        # Delete gateway policy
        policy_prefix = f"{self.name_prefix} Block Ads"
        deleted_policies = cloudflare.delete_gateway_policy(policy_prefix)
        logging.info(f"Deleted {deleted_policies} gateway policies")

        # Delete lists
        cf_lists = cloudflare.get_lists(self.name_prefix)
        for l in cf_lists:
            logging.info(f"Deleting list {l['name']} - ID:{l['id']} ")
            cloudflare.delete_list(l["name"], l["id"])

        logging.info("Deletion completed")

    def write_list(self):
        file_content = ""
        whitelist_content = ""
        for url in self.adlist_urls:
            file_content += self.download_file(url)
        for url in self.whitelist_urls:
            whitelist_content += self.download_file(url)
        domains = self.convert_to_domain_list(file_content)
        whitelist_domains = self.convert_to_domain_list(whitelist_content)

        # remove whitelisted domains
        filtered_domains = list(set(domains) - set(whitelist_domains))
        logging.info(f"Number of domains after filtering: {len(filtered_domains)}")
