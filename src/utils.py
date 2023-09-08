import asyncio
import logging
import re

import aiohttp

from src import cloudflare

simple_ip_regex = re.compile(r"^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\s*(.+)$")

valid_domain_regex = re.compile(r"^((?!-)[A-Za-z0-9-]{1,63}(?<!-)\.)+[A-Za-z]{2,6}$")


class App:
    def __init__(
        self,
        adlist_name: str,
        adlist_urls: list[str],
        whitelist_urls: list[str],
    ):
        self.adlist_name = adlist_name
        self.adlist_urls = adlist_urls
        self.whitelist_urls = whitelist_urls
        self.name_prefix = f"[AdBlock-{adlist_name}]"

    async def run(self):
        domains = await self.get_domains()

        # check if the list is already in Cloudflare
        cf_lists = await cloudflare.get_lists(self.name_prefix)

        logging.info(f"Number of lists in Cloudflare: {len(cf_lists)}")

        # compare the lists size
        if len(domains) == sum([l["count"] for l in cf_lists]):
            logging.warning("Lists are the same size, skipping")
            return

        # Delete existing policy created by script
        policy_prefix = f"{self.name_prefix} Block Ads"
        deleted_policies = await cloudflare.delete_gateway_policy(policy_prefix)
        logging.info(f"Deleted {deleted_policies} gateway policies")

        # Delete existing lists created by script
        delete_list_tasks = []
        for l in cf_lists:
            logging.info(f"Deleting list {l['name']} - ID:{l['id']} ")
            delete_list_tasks.append(
                asyncio.create_task(cloudflare.delete_list(l["name"], l["id"]))
            )
        for task in delete_list_tasks:
            await task

        # chunk the domains into lists of 1000 and create them
        create_list_tasks = []
        for i, chunk in enumerate(self.chunk_list(domains, 1000)):
            list_name = f"{self.name_prefix} {i + 1}"
            logging.info(f"Creating list {list_name}")
            create_list_tasks.append(
                asyncio.create_task(cloudflare.create_list(list_name, chunk))
            )
        cf_lists = []
        for task in create_list_tasks:
            _list = await task
            if _list:
                cf_lists.append(_list)

        # get the gateway policies
        cf_policies = await cloudflare.get_firewall_policies(self.name_prefix)

        logging.info(f"Number of policies in Cloudflare: {len(cf_policies)}")

        # setup the gateway policy
        if len(cf_policies) == 0:
            logging.info("Creating firewall policy")
            cf_policies = await cloudflare.create_gateway_policy(
                f"{self.name_prefix} Block Ads",
                [l["id"] for l in cf_lists],
            )

        elif len(cf_policies) != 1:
            logging.error("More than one firewall policy found")
            raise Exception("More than one firewall policy found")

        else:
            logging.info("Updating firewall policy")
            await cloudflare.update_gateway_policy(
                f"{self.name_prefix} Block Ads",
                cf_policies[0]["id"],
                [l["id"] for l in cf_lists],
            )

        logging.info("Done")

    def convert_to_domain_set(self, file_content: str):
        skip_domains = [
            "localhost",
            "local",
            "localhost.localdomain",
        ]

        domains = set()

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

            domains.add(domain.encode("idna").decode())

        logging.info(f"Number of domains: {len(domains)}")

        return domains

    @staticmethod
    def chunk_list(_list: list[str], n: int):
        for i in range(0, len(_list), n):
            yield _list[i : i + n]

    async def delete(self):
        # Delete gateway policy
        policy_prefix = f"{self.name_prefix} Block Ads"
        deleted_policies = await cloudflare.delete_gateway_policy(policy_prefix)
        logging.info(f"Deleted {deleted_policies} gateway policies")

        # Delete lists
        cf_lists = await cloudflare.get_lists(self.name_prefix)

        delete_lists_tasks = []
        for l in cf_lists:
            logging.info(f"Deleting list {l['name']} - ID:{l['id']} ")
            delete_lists_tasks.append(
                asyncio.create_task(cloudflare.delete_list(l["name"], l["id"]))
            )
        for task in delete_lists_tasks:
            await task

        logging.info("Deletion completed")

    async def write_list(self):
        filtered_domains = await self.get_domains()
        with open("domains.txt", "w") as f:
            for item in filtered_domains:
                f.write("%s\n" % item)

    async def get_domains(self):
        async with aiohttp.ClientSession() as session:
            file_content = "".join(
                await asyncio.gather(
                    *[
                        self.download_file_async(session, url)
                        for url in self.adlist_urls
                    ]
                )
            )
            whitelist_content = "".join(
                await asyncio.gather(
                    *[
                        self.download_file_async(session, url)
                        for url in self.whitelist_urls
                    ]
                )
            )
        domains = self.convert_to_domain_set(file_content)
        whitelist_domains = self.convert_to_domain_set(whitelist_content)

        # remove whitelisted domains
        filtered_domains = sorted(list(domains - whitelist_domains))
        logging.info(f"Number of domains after filtering: {len(filtered_domains)}")
        return filtered_domains

    async def download_file_async(self, session: aiohttp.ClientSession, url: str):
        async with session.get(url) as response:
            text = await response.text()
            # Workaround for stevenblack
            if "# Start StevenBlack" in text:
                text = text.split("# Start StevenBlack")[1]
            logging.info(f"Downloaded file from {url}. File size: {len(text)}")
            return text
