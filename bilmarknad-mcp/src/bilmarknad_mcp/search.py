from __future__ import annotations

import os
from typing import Any

from bilmarknad_mcp.blocket import BlocketClient, build_params
from bilmarknad_mcp.blocket_proxy import BlocketProxyClient
from bilmarknad_mcp.kvd import KvdClient, KvdUnavailableError
from bilmarknad_mcp.schema import CarListing
from bilmarknad_mcp.urls import parse_listing_url
from bilmarknad_mcp.tradera import TraderaClient, TraderaUnavailableError
from bilmarknad_mcp.wayke import WaykeClient
from bilmarknad_mcp.riddermark import RiddermarkClient
from bilmarknad_mcp.carla import CarlaClient

ALL_SOURCES = ("blocket", "wayke", "kvd", "tradera", "riddermark", "carla")

def normalize_sources(sources):
    if not sources:
        return list(ALL_SOURCES)
    out = []
    for raw in sources:
        key = (raw or "").strip().lower()
        if key in ALL_SOURCES and key not in out:
            out.append(key)
    return out or list(ALL_SOURCES)

_normalize_sources = normalize_sources

def listing_key(item):
    return (item.source, str(item.id))

def dedupe_listings(items):
    seen = set()
    out = []
    for item in items:
        key = listing_key(item)
        if key in seen:
            continue
        seen.add(key)
        out.append(item)
    return out

def matches_filters(item, make, model, price_min, price_max, year_min, year_max, mileage_max_km):
    if make and item.make and make.strip().lower() not in item.make.lower():
        return False
    if model and item.model and model.strip().lower() not in item.model.lower():
        return False
    if price_min is not None and item.price_sek is not None and item.price_sek < price_min:
        return False
    if price_max is not None and item.price_sek is not None and item.price_sek > price_max:
        return False
    if year_min is not None and item.year is not None and item.year < year_min:
        return False
    if year_max is not None and item.year is not None and item.year > year_max:
        return False
    if mileage_max_km is not None and item.mileage_km is not None and item.mileage_km > mileage_max_km:
        return False
    return True


class SearchService:
    def __init__(self, use_blocket_proxy=False):
        self.use_blocket_proxy = use_blocket_proxy
        self.blocket = None
        self.wayke = None
        self.kvd = None
        self.tradera = None
        self.riddermark = None
        self.carla = None

    def blocket_client(self):
        if self.blocket is None:
            if self.use_blocket_proxy:
                self.blocket = BlocketProxyClient()
            else:
                self.blocket = BlocketClient()
        return self.blocket


    def wayke_client(self):
        if self.wayke is None:
            self.wayke = WaykeClient()
        return self.wayke

    def kvd_client(self):
        if self.kvd is None:
            self.kvd = KvdClient()
        return self.kvd

    def tradera_client(self):
        if self.tradera is None:
            self.tradera = TraderaClient()
        return self.tradera

    def riddermark_client(self):
        if self.riddermark is None:
            self.riddermark = RiddermarkClient()
        return self.riddermark

    def carla_client(self):
        if self.carla is None:
            self.carla = CarlaClient()
        return self.carla

    def close(self):
        for client in (self.blocket, self.wayke, self.kvd, self.tradera, self.riddermark, self.carla):
            if client is not None:
                client.close()

    def search_cars(self, **kw):
        query = kw.get('query')
        make = kw.get('make')
        model = kw.get('model')
        price_min = kw.get('price_min')
        price_max = kw.get('price_max')
        year_min = kw.get('year_min')
        year_max = kw.get('year_max')
        mileage_max_km = kw.get('mileage_max_km')
        fuel_types = kw.get('fuel_types')
        transmission = kw.get('transmission')
        sources = kw.get('sources')
        limit = kw.get('limit') or 20
        page = kw.get('page') or 1
        sort = kw.get('sort')
        active = normalize_sources(sources)
        collected=list()
        for source in active:
            if source=="blocket":
                for fuel in (fuel_types or [None]):
                    client=self.blocket_client()
                    bp={}
                    bp.update(q=query,make=make,model=model,price_from=price_min,price_to=price_max,year_from=year_min,year_to=year_max,mileage_to_km=mileage_max_km,fuel=fuel,transmission=transmission,sort=sort,rows=limit,page=page)
                    if isinstance(client,BlocketProxyClient):
                        collected.extend(client.search(**build_params(**bp)))
                    else:
                        collected.extend(client.search(**bp))
            elif source=="wayke":
                parts=[p for p in (query,make,model) if p]
                q=" ".join(parts) if parts else None
                collected.extend(self.wayke_client().search(q=q,rows=limit,page=page))
            elif source=="kvd":
                try:
                    collected.extend(self.kvd_client().search())
                except KvdUnavailableError:
                    pass
            elif source=="tradera":
                parts=[p for p in (query,make,model) if p]
                q=" ".join(parts) if parts else None
                try:
                    collected.extend(self.tradera_client().search(q=q,rows=limit,page=page))
                except TraderaUnavailableError:
                    pass
            elif source=="riddermark":
                collected.extend(
                    self.riddermark_client().search(
                        q=query,
                        make=make,
                        model=model,
                        price_min=price_min,
                        price_max=price_max,
                        mileage_max_km=mileage_max_km,
                        rows=limit,
                        page=page,
                    )
                )
            elif source=="carla":
                fuel = (fuel_types or [None])[0]
                collected.extend(
                    self.carla_client().search(
                        q=query,
                        make=make,
                        model=model,
                        fuel=fuel,
                        rows=limit,
                        page=page,
                    )
                )
        filtered=[item for item in collected if matches_filters(item,make,model,price_min,price_max,year_min,year_max,mileage_max_km)]
        deduped=dedupe_listings(filtered)
        return [item.to_dict() for item in deduped[:limit]]

    def get_listing(self,source=None,listing_id=None,url=None):
        if url:
            parsed=parse_listing_url(url)
            if parsed:
                source,listing_id=parsed
        if not source or not listing_id:
            return None
        src=source.strip().lower()
        if src=="blocket":
            item=self.blocket_client().get_listing(listing_id)
            return item.to_dict() if item else None
        if src=="wayke":
            item=self.wayke_client().get_vehicle(listing_id)
            return item.to_dict() if item else None
        if src=="kvd":
            try:
                items=self.kvd_client().search()
            except KvdUnavailableError:
                return None
            for item in items:
                if str(item.id)==str(listing_id):
                    return item.to_dict()
            return None
        if src=="tradera":
            try:
                item=self.tradera_client().get_listing(listing_id)
            except TraderaUnavailableError:
                return None
            return item.to_dict() if item else None
        if src=="riddermark":
            item=self.riddermark_client().get_listing(listing_id)
            return item.to_dict() if item else None
        if src=="carla":
            item=self.carla_client().get_listing(listing_id)
            return item.to_dict() if item else None
        return None

    def list_sources(self):
        return {"sources":[{"id":"blocket","description":"Blocket mobility used-car search API","env":["BLOCKET_PROXY_URL"]},{"id":"wayke","description":"Wayke vehicle search (REST with API key or public GraphQL)","env":["WAYKE_API_KEY"]},{"id":"kvd","description":"KVD public API probe (returns empty until a stable endpoint exists)","env":[]},{"id":"tradera","description":"Tradera car auctions and buy-now listings (REST API v3, 100 calls/day)","env":["TRADERA_APP_ID","TRADERA_APP_KEY","TRADERA_CAR_CATEGORY_ID"]},{"id":"riddermark","description":"Riddermark Bil used-car search via Next.js page data","env":[]},{"id":"carla","description":"Carla EV marketplace search via Next.js data API","env":[]}],"env":{"WAYKE_API_KEY":{"required":False,"description":"Optional Wayke REST API bearer token","set":bool(os.environ.get("WAYKE_API_KEY"))},"BLOCKET_PROXY_URL":{"required":False,"description":"Optional Blocket search proxy base URL","set":bool(os.environ.get("BLOCKET_PROXY_URL"))},"TRADERA_APP_ID":{"required":False,"description":"Tradera developer app ID (defaults to shared dev credentials)","set":bool(os.environ.get("TRADERA_APP_ID"))},"TRADERA_APP_KEY":{"required":False,"description":"Tradera developer app key (defaults to shared dev credentials)","set":bool(os.environ.get("TRADERA_APP_KEY"))},"TRADERA_CAR_CATEGORY_ID":{"required":False,"description":"Tradera category ID for car searches (default 10 = Bilar)","set":bool(os.environ.get("TRADERA_CAR_CATEGORY_ID"))}}}
