import logging
import traceback

from src.utils import App
from src.colorlogs import ColoredLevelFormatter

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


if __name__ == "__main__":
    adlist_urls = read_domain_urls()
    adlist_name = "DNS Block List"
    app = App(adlist_name, adlist_urls)
    try:
        app.run()
    except Exception:
        traceback.print_exc()
        exit(1)
