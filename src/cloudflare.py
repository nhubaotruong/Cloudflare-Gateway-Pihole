import os

import requests
from dotenv import load_dotenv

load_dotenv()

CF_API_TOKEN = os.getenv("CF_API_TOKEN") or os.environ.get("CF_API_TOKEN")
CF_IDENTIFIER = os.getenv("CF_IDENTIFIER") or os.environ.get("CF_IDENTIFIER")

if not CF_API_TOKEN or not CF_IDENTIFIER:
    raise Exception("Missing Cloudflare credentials")

session = requests.Session()
session.headers.update({"Authorization": f"Bearer {CF_API_TOKEN}"})


def get_lists(name_prefix: str):
    resp = session.get(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/lists",
    )

    if resp.status_code != 200:
        raise Exception(resp.text)

    lists = resp.json()["result"] or []

    return [l for l in lists if l["name"].startswith(name_prefix)]


def create_list(name: str, domains: list[str]):
    resp = session.post(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/lists",
        json={
            "name": name,
            "description": "Created by script.",
            "type": "DOMAIN",
            "items": [{"value": d} for d in domains],
        },
    )

    if resp.status_code != 200:
        json_body = resp.json()
        if json_body["errors"][0]["code"] == 1204:
            return
        else:
            raise Exception(resp.text)

    return resp.json()["result"]


def delete_list(name: str, list_id: str):
    resp = session.delete(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/lists/{list_id}",
    )

    if resp.status_code != 200:
        raise Exception(resp.text)

    return resp.json()["result"]


def get_firewall_policies(name_prefix: str):
    resp = session.get(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules",
    )

    if resp.status_code != 200:
        raise Exception(resp.text)
    lists = resp.json()["result"] or []
    return [l for l in lists if l["name"].startswith(name_prefix)]


def create_gateway_policy(name: str, list_ids: list[str]):
    resp = session.post(
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
    )

    if resp.status_code != 200:
        raise Exception(resp.text)
    return resp.json()["result"]


def update_gateway_policy(name: str, policy_id: str, list_ids: list[str]):
    resp = session.put(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules/{policy_id}",
        json={
            "name": name,
            "action": "block",
            "enabled": True,
            "traffic": "or".join([f"any(dns.domains[*] in ${l})" for l in list_ids]),
        },
    )

    if resp.status_code != 200:
        raise Exception(resp.text)
    return resp.json()["result"]


def delete_gateway_policy(policy_name_prefix: str):
    resp = session.get(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules",
    )

    if resp.status_code != 200:
        raise Exception(resp.text)

    policies = resp.json()["result"] or []
    policy_to_delete = next(
        (p for p in policies if p["name"].startswith(policy_name_prefix)), None
    )

    if not policy_to_delete:
        return 0

    policy_id = policy_to_delete["id"]

    resp = session.delete(
        f"https://api.cloudflare.com/client/v4/accounts/{CF_IDENTIFIER}/gateway/rules/{policy_id}",
    )

    if resp.status_code != 200:
        raise Exception(resp.text)
    return 1
