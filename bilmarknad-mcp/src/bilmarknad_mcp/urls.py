from __future__ import annotations

import re
from urllib.parse import urlparse

_BLOCKET_ITEM = re.compile(r"/mobility/item/(?P<id>\d+)", re.I)
_WAYKE_VEHICLE = re.compile(r"/(?:vehicle|fordon|bilar)/(?P<id>[^/?#]+)", re.I)
_KVD_OBJECT = re.compile(r"/(?:objekt|vehicle|bil|auktion)/(?P<id>\d+)", re.I)
_TRADERA_ITEM = re.compile(r"/item/(?P<id>\d+)", re.I)
_RIDDERMARK_CAR = re.compile(r"/kopa-bil/(?P<id>[^/?#]+/[^/?#]+)", re.I)
_CARLA_CAR = re.compile(r"/bil/(?P<id>[^/?#]+)", re.I)


def parse_listing_url(url: str) -> tuple[str, str] | None:
    """Detect marketplace source and listing id from a public listing URL."""
    raw = (url or "").strip()
    if not raw:
        return None
    parsed = urlparse(raw)
    host = (parsed.netloc or "").lower().removeprefix("www.")
    path = parsed.path or ""

    if host.endswith("blocket.se"):
        match = _BLOCKET_ITEM.search(path)
        if match:
            return ("blocket", match.group("id"))

    if host.endswith("wayke.se"):
        match = _WAYKE_VEHICLE.search(path)
        if match:
            return ("wayke", match.group("id"))

    if host.endswith("kvd.se"):
        match = _KVD_OBJECT.search(path)
        if match:
            return ("kvd", match.group("id"))

    if host.endswith("tradera.com") or host.endswith("tradera.se"):
        match = _TRADERA_ITEM.search(path)
        if match:
            return ("tradera", match.group("id"))

    if host.endswith("riddermarkbil.se"):
        match = _RIDDERMARK_CAR.search(path)
        if match:
            return ("riddermark", match.group("id").strip("/").lower())

    if host.endswith("carla.se"):
        match = _CARLA_CAR.search(path)
        if match:
            return ("carla", match.group("id").strip("/"))

    return None
