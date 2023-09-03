import logging
import traceback

from src.colorlogs import ColoredLevelFormatter
from src.utils import App

logging.getLogger().setLevel(logging.INFO)
formatter = ColoredLevelFormatter("%(levelname)s: %(message)s")
console = logging.StreamHandler()
console.setFormatter(ColoredLevelFormatter("%(levelname)s: %(message)s"))
logger = logging.getLogger()
logger.addHandler(console)


def read_domain_urls():
    with open("lists.txt", "r") as file:
        adlist_urls = [url.strip() for url in file if url.strip()]
    return adlist_urls


def read_whitelist_urls():
    with open("whitelists.txt", "r") as file:
        whitelist_urls = [url.strip() for url in file if url.strip()]
    return whitelist_urls


if __name__ == "__main__":
    adlist_urls = read_domain_urls()
    whitelist_urls = read_whitelist_urls()
    adlist_name = "DNS Block List"
    app = App(adlist_name, adlist_urls, whitelist_urls)
    try:
        app.run()
    except Exception:
        traceback.print_exc()
        exit(1)
