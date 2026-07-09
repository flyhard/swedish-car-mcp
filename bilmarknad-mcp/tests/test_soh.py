from bilmarknad_mcp.blocket import parse_ad
from bilmarknad_mcp.soh import extract_soh_from_text


def test_extract_soh_percent_from_soh_label():
    result = extract_soh_from_text("Batteri SOH 92,5% enligt test")
    assert result["soh_percent"] == 92.5
    assert result["battery_tested"] is True
    assert result["soh_raw_match"] is not None


def test_extract_soh_percent_suffix():
    result = extract_soh_from_text("92% batterihälsa")
    assert result["soh_percent"] == 92.0
    assert result["battery_tested"] is True


def test_extract_soh_battery_tested_only():
    result = extract_soh_from_text("Bilen är batteritestad hos Aviloo")
    assert result["soh_percent"] is None
    assert result["battery_tested"] is True


def test_extract_soh_empty():
    result = extract_soh_from_text(None)
    assert result["soh_percent"] is None
    assert result["battery_tested"] is False


def test_parse_ad_model_specification_soh():
    listing = parse_ad(
        {
            "ad_id": 42,
            "heading": "Kia e-Niro",
            "mileage": 100,
            "price": {"amount": 250000},
            "model_specification": "SOH 88% batterihälsa",
        }
    )
    assert listing.soh_percent == 88.0
    assert listing.battery_tested is True
    assert listing.soh_source == "blocket_search"

def test_apply_soh_preserves_percent_on_battery_only_enrich():
    from bilmarknad_mcp.schema import CarListing
    from bilmarknad_mcp.soh import apply_soh

    listing = CarListing(source="blocket", id="1", title="EV")
    apply_soh(listing, "SOH 88%", source="blocket_search")
    apply_soh(listing, "Batteritestad", source="blocket_detail")
    assert listing.soh_percent == 88.0
    assert listing.battery_tested is True
    assert listing.soh_source == "blocket_search"

