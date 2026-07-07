from __future__ import annotations

import json
from mcp.server.fastmcp import FastMCP

from bilmarknad_mcp.search import SearchService

mcp = FastMCP("bilmarknad")
_service = SearchService()


@mcp.tool()
def search_cars(query=None, q=None, make=None, model=None, price_min=None, price_max=None, year_min=None, year_max=None, mileage_max_km=None, fuel_types=None, transmission=None, sources=None, limit=None, page=None, sort=None, use_blocket_proxy=False):
    """Search used cars across Swedish marketplaces."""
    text_query = query if query is not None else q
    svc = _service if not use_blocket_proxy else SearchService(use_blocket_proxy=True)
    try:
        results = svc.search_cars(query=text_query, make=make, model=model, price_min=price_min, price_max=price_max, year_min=year_min, year_max=year_max, mileage_max_km=mileage_max_km, fuel_types=fuel_types, transmission=transmission, sources=sources, limit=limit or 20, page=page or 1, sort=sort)
    finally:
        if use_blocket_proxy:
            svc.close()
    payload = {"count": len(results), "listings": results}
    return json.dumps(payload, ensure_ascii=False, indent=2)

@mcp.tool()
def get_listing(source=None, id=None, listing_id=None, url=None):
    """Fetch one listing by source+id or by public listing URL."""
    lid = id if id is not None else listing_id
    item = _service.get_listing(source=source, listing_id=lid, url=url)
    if item is not None:
        return json.dumps(item, ensure_ascii=False, indent=2)
    not_found = {"found": False, "source": source, "id": lid, "url": url}
    return json.dumps(not_found, ensure_ascii=False, indent=2)

@mcp.tool()
def list_sources():
    """List supported sources and related environment variables."""
    return json.dumps(_service.list_sources(), ensure_ascii=False, indent=2)

def main() -> None:
    mcp.run()

if __name__ == "__main__":
    main()
