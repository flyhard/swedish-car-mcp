"""Tests for AVILOO text parsing."""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from aviloo_mcp.parser import parse_aviloo_text

SAMPLE_SNIPPET = """
CERTIFIKATETS NUMMER: C94D3EFA-2493-4ED3-B9FC-96DD6754575F
VARUMÄRKE: Kia
MODELL: Niro EV - 64,8 kWh
MÄTARSTÄLLNING: 57 265 km
VIN: KNACR811FP5026133
DATUM OCH TID:
2026-05-22 09:43
HÄLSOTILLSTÅND (SOH)
97,4 %
63kWh | 65kWh
448km | 460km
UTFÖRD AV: Riddermark Bil AB
UTMÄRKT HÄLSA – INGA AVVIKELSER UPPTÄCKTA
officiellt AVILOO-certifierat
"""

UUID_RE = re.compile(
    r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"
)


def test_uuid_regex():
    assert UUID_RE.match("C94D3EFA-2493-4ED3-B9FC-96DD6754575F")
    assert not UUID_RE.match("not-a-uuid")


def test_soh_from_snippet():
    parsed = parse_aviloo_text(SAMPLE_SNIPPET, download_id="cae9d182733848a7873983fe2b2264a0")
    assert parsed["soh_percent"] == pytest.approx(97.4)
    assert parsed["certificate_number"] == "C94D3EFA-2493-4ED3-B9FC-96DD6754575F"
    assert parsed["vin"] == "KNACR811FP5026133"
    assert parsed["mileage_km"] == 57265
    assert parsed["certified"] is True


def test_repo_root_sample_if_present():
    pdf = ROOT / "cae9d182733848a7873983fe2b2264a0.pdf"
    if not pdf.is_file():
        pytest.skip("sample file not in repo root")
    exe = os.environ.get("PDFTOTEXT", "pdftotext")
    try:
        proc = subprocess.run(
            [exe, str(pdf), "-"],
            capture_output=True,
            text=True,
            check=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError):
        pytest.skip("pdftotext unavailable or failed")
    parsed = parse_aviloo_text(proc.stdout, download_id=pdf.stem)
    assert parsed["soh_percent"] == pytest.approx(97.4, rel=0, abs=0.1)
    assert parsed["certificate_number"] == "C94D3EFA-2493-4ED3-B9FC-96DD6754575F"
