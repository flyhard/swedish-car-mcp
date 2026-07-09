from __future__ import annotations

import re
from typing import Any

from bilmarknad_mcp.schema import CarListing

_SOH_PERCENT_PATTERNS = [
    re.compile(r"(?i)(?:soh|hälsotillstånd|batterihälsa|state\s+of\s+health)[^\d]{0,20}(\d{1,3}(?:[.,]\d{1,2})?)\s*%"),
    re.compile(r"(?i)(\d{1,3}(?:[.,]\d{1,2})?)\s*%\s*(?:soh|batterihälsa|hälsotillstånd)"),
    re.compile(r"(?i)(\d{1,3}(?:[.,]\d{1,2})?)\s*%\s*(?:batteri)?hälsa"),
    re.compile(r"(?i)hälsotillstånd\s*\(soh\)[^\d]{0,20}(\d{1,3}(?:[.,]\d{1,2})?)\s*%"),
    re.compile(r"(?i)pro\s+soh\s+(\d{1,3}(?:[.,]\d{1,2})?)\s*%"),
]

_BATTERY_TESTED_PATTERNS = [
    re.compile(r"(?i)\bbatteritestad\b"),
    re.compile(r"(?i)\baviloo\b"),
    re.compile(r"(?i)\bhälsotillstånd\b"),
    re.compile(r"(?i)\bsoh\b"),
    re.compile(r"(?i)\bbatterihälsa\b"),
]

def _normalize_percent(raw: str) -> float | None:
    try:
        value = float(raw.replace(",", "."))
    except ValueError:
        return None
    if 0 < value <= 100:
        return value
    return None

def extract_soh_from_text(text: str | None) -> dict[str, Any]:
    result: dict[str, Any] = {"soh_percent": None, "battery_tested": False, "soh_raw_match": None}
    if not text or not str(text).strip():
        return result
    text_str = str(text)
    for pattern in _BATTERY_TESTED_PATTERNS:
        if pattern.search(text_str):
            result["battery_tested"] = True
            break
    for pattern in _SOH_PERCENT_PATTERNS:
        match = pattern.search(text_str)
        if match:
            percent = _normalize_percent(match.group(1))
            if percent is not None:
                result["soh_percent"] = percent
                result["soh_raw_match"] = match.group(0)
                return result
    return result

def extract_soh_from_fields(*fields: str | None) -> dict[str, Any]:
    combined: dict[str, Any] = {"soh_percent": None, "battery_tested": False, "soh_raw_match": None}
    for field in fields:
        if not field:
            continue
        found = extract_soh_from_text(field)
        if found["battery_tested"]:
            combined["battery_tested"] = True
        if combined["soh_percent"] is None and found["soh_percent"] is not None:
            combined["soh_percent"] = found["soh_percent"]
            combined["soh_raw_match"] = found["soh_raw_match"]
    return combined

def apply_soh(listing: CarListing, *fields: str | None, source: str) -> CarListing:
    soh = extract_soh_from_fields(*fields)
    if soh["soh_percent"] is None and not soh["battery_tested"]:
        return listing
    if soh["soh_percent"] is not None:
        listing.soh_percent = soh["soh_percent"]
        listing.soh_raw_match = soh["soh_raw_match"]
        listing.soh_source = source
    if soh["battery_tested"]:
        listing.battery_tested = True
    return listing
