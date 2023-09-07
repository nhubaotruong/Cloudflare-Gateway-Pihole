import asyncio
import logging
import os
import traceback

import aiohttp
from dotenv import load_dotenv

from src.colorlogs import ColoredLevelFormatter
from src.utils import App

logging.getLogger().setLevel(logging.INFO)
formatter = ColoredLevelFormatter("%(levelname)s: %(message)s")
console = logging.StreamHandler()
console.setFormatter(ColoredLevelFormatter("%(levelname)s: %(message)s"))
logger = logging.getLogger()
logger.addHandler(console)


load_dotenv()

CF_API_TOKEN = os.getenv("CF_API_TOKEN") or os.environ.get("CF_API_TOKEN")
CF_IDENTIFIER = os.getenv("CF_IDENTIFIER") or os.environ.get("CF_IDENTIFIER")

if not CF_API_TOKEN or not CF_IDENTIFIER:
    raise Exception("Missing Cloudflare credentials")


def read_domain_urls():
    with open("lists.txt", "r") as file:
        adlist_urls = [url.strip() for url in file if url.strip()]
    return adlist_urls


def read_whitelist_urls():
    with open("whitelists.txt", "r") as file:
        whitelist_urls = [url.strip() for url in file if url.strip()]
    return whitelist_urls


async def main():
    adlist_urls = read_domain_urls()
    whitelist_urls = read_whitelist_urls()
    adlist_name = "DNS Block List"

    async with aiohttp.ClientSession(
        headers={"Authorization": f"Bearer {CF_API_TOKEN}"}
    ) as session:
        try:
            app = App(adlist_name, adlist_urls, whitelist_urls, session)
            await app.run()
        except Exception:
            traceback.print_exc()
            exit(1)


if __name__ == "__main__":
    asyncio.run(main())
