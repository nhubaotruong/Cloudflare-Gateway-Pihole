import os

import aiohttp
from dotenv import load_dotenv

load_dotenv()

CF_IDENTIFIER = os.getenv("CF_IDENTIFIER") or os.environ.get("CF_IDENTIFIER")


async def get_lists(session: aiohttp.ClientSession, name_prefix: str):
    async with session.get(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/lists",
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        lists = (await resp.json())["result"] or []
        return [l for l in lists if l["name"].startswith(name_prefix)]


async def create_list(session: aiohttp.ClientSession, name: str, domains: list[str]):
    async with session.post(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/lists",
        json={
            "name": name,
            "description": "Created by script.",
            "type": "DOMAIN",
            "items": [{"value": d} for d in domains],
        },
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        return (await resp.json())["result"]


async def delete_list(session: aiohttp.ClientSession, name: str, list_id: str):
    async with session.delete(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/lists/{list_id}",
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        return (await resp.json())["result"]


async def get_firewall_policies(session: aiohttp.ClientSession, name_prefix: str):
    async with session.get(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules",
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        policies = (await resp.json())["result"] or []
        return [l for l in policies if l["name"].startswith(name_prefix)]


async def create_gateway_policy(
    session: aiohttp.ClientSession, name: str, list_ids: list[str]
):
    async with session.post(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules",
        json={
            "name": name,
            "description": "Created by script.",
            "action": "block",
            "enabled": True,
            "filters": ["dns"],
            "traffic": "or".join([f"any(dns.domains[*] in ${l})" for l in list_ids]),
            "rule_settings": {
                "block_page_enabled": False,
            },
        },
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        return (await resp.json())["result"]


async def update_gateway_policy(
    session: aiohttp.ClientSession, name: str, policy_id: str, list_ids: list[str]
):
    async with session.put(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules/{policy_id}",
        json={
            "name": name,
            "action": "block",
            "enabled": True,
            "traffic": "or".join([f"any(dns.domains[*] in ${l})" for l in list_ids]),
        },
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        return (await resp.json())["result"]


async def delete_gateway_policy(
    session: aiohttp.ClientSession, policy_name_prefix: str
):
    async with session.get(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules",
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        policies = (await resp.json())["result"] or []
        policy_to_delete = next(
            (p for p in policies if p["name"].startswith(policy_name_prefix)), None
        )

        if not policy_to_delete:
            return 0

        policy_id = policy_to_delete["id"]

    async with session.delete(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules/{policy_id}",
    ) as resp:
        if resp.status != 200:
            raise Exception(await resp.text())

        return 1
