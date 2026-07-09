from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any


@dataclass
class CarListing:
    source: str
    id: str
    title: str
    make: str | None = None
    model: str | None = None
    year: int | None = None
    mileage_km: int | None = None
    price_sek: int | None = None
    fuel: str | None = None
    transmission: str | None = None
    location: str | None = None
    dealer_name: str | None = None
    url: str | None = None
    image_url: str | None = None
    published_at: str | None = None
    registration_number: str | None = None
    soh_percent: float | None = None
    battery_tested: bool = False
    soh_source: str | None = None
    soh_raw_match: str | None = None
    raw: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

