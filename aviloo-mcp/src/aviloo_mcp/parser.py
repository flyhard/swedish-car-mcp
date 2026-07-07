"""Extract and parse AVILOO battery certificate documents (Swedish layout)."""

from __future__ import annotations

import re
import shutil
import subprocess
from pathlib import Path

_UUID_RE = re.compile(
    r"([0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12})"
)
_SOH_LABEL_RE = re.compile(
    r"H[\u00c4A]LSOTILLST[\u00c4A]ND\s*\(SOH\)[\s\S]{0,400}?(\d{1,3}(?:[.,]\d+)?)\s*%",
    re.IGNORECASE,
)
_PERCENT_RE = re.compile(r"(\d{1,3}(?:[.,]\d+)?)\s*%")
_ENERGY_RE = re.compile(
    r"(\d+(?:[.,]\d+)?)\s*kWh\s*\|\s*(\d+(?:[.,]\d+)?)\s*kWh",
    re.IGNORECASE,
)
_WLTP_RE = re.compile(
    r"(\d+(?:[.,]\d+)?)\s*km\s*\|\s*(\d+(?:[.,]\d+)?)\s*km",
    re.IGNORECASE,
)


def _pdftotext(pdf_path: Path) -> str:
    exe = shutil.which("pdftotext")
    if not exe:
        raise RuntimeError("pdftotext not found on PATH")
    result = subprocess.run(
        [exe, str(pdf_path), "-"],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"pdftotext failed ({result.returncode}): {result.stderr.strip()}"
        )
    return result.stdout


def _pypdf_text(pdf_path: Path) -> str:
    try:
        from pypdf import PdfReader
    except ImportError as exc:
        raise RuntimeError("pdftotext missing and pypdf not installed") from exc
    reader = PdfReader(str(pdf_path))
    parts: list[str] = []
    for page in reader.pages:
        parts.append(page.extract_text() or "")
    return chr(10).join(parts)


def extract_text(pdf_path: Path) -> str:
    """Extract plain text using pdftotext, optional pypdf fallback."""
    pdf_path = Path(pdf_path)
    if shutil.which("pdftotext"):
        return _pdftotext(pdf_path)
    return _pypdf_text(pdf_path)


def _parse_float(s: str) -> float:
    return float(s.replace(",", ".").replace(" ", ""))


def _parse_int_km(s: str) -> int:
    return int(re.sub(r"[^\d]", "", s))


def _field_after(text: str, label_pattern: str) -> str | None:
    m = re.search(label_pattern + r"\s*(.+)", text, re.IGNORECASE)
    if not m:
        return None
    line = m.group(1).strip()
    return line.split(chr(10))[0].strip() or None


def parse_aviloo_text(text: str, download_id: str | None = None) -> dict:
    """Parse AVILOO certificate fields from extracted text."""
    raw_soh_matches = [m.group(0) for m in _PERCENT_RE.finditer(text)]

    certificate_number: str | None = None
    cert_line = re.search(
        r"CERTIFIKATETS\s+NUMMER:\s*(" + _UUID_RE.pattern + r")",
        text,
        re.IGNORECASE,
    )
    if cert_line:
        certificate_number = cert_line.group(1).upper()
    else:
        m = _UUID_RE.search(text)
        if m:
            certificate_number = m.group(1).upper()

    brand = _field_after(text, r"VARUM[\u00c4A]RKE:")
    model = _field_after(text, r"MODELL:")
    vin = _field_after(text, r"VIN:")
    mileage_raw = _field_after(text, r"M[\u00c4A]TARST[\u00c4A]LLNING:")
    mileage_km = _parse_int_km(mileage_raw) if mileage_raw else None

    tested_at: str | None = None
    dt = re.search(
        r"DATUM\s+OCH\s+TID:\s*\n?\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2})",
        text,
        re.IGNORECASE,
    )
    if dt:
        tested_at = dt.group(1).strip()

    performed_by = _field_after(text, r"UTF[\u00d6O]RD\s+AV:")

    soh_percent: float | None = None
    soh_m = _SOH_LABEL_RE.search(text)
    if soh_m:
        soh_percent = _parse_float(soh_m.group(1))
    else:
        for pm in _PERCENT_RE.finditer(text):
            val = _parse_float(pm.group(1))
            if 50 <= val <= 100:
                soh_percent = val
                break

    energy_current_kwh: float | None = None
    energy_new_kwh: float | None = None
    em = _ENERGY_RE.search(text)
    if em:
        energy_current_kwh = _parse_float(em.group(1))
        energy_new_kwh = _parse_float(em.group(2))

    wltp_current_km: float | None = None
    wltp_new_km: float | None = None
    wm = _WLTP_RE.search(text)
    if wm:
        wltp_current_km = _parse_float(wm.group(1))
        wltp_new_km = _parse_float(wm.group(2))

    assessment: str | None = None
    if re.search(r"UTM[\u00c4A]RKT\s+H[\u00c4A]LSA", text, re.IGNORECASE):
        assessment = "UTM\u00c4RKT H\u00c4LSA"
    certified = bool(
        re.search(r"AVILOO-certifierat", text, re.IGNORECASE)
        or re.search(r"officiellt\s+AVILOO", text, re.IGNORECASE)
    )

    return {
        "certificate_number": certificate_number,
        "download_id": download_id,
        "soh_percent": soh_percent,
        "vin": vin,
        "mileage_km": mileage_km,
        "tested_at": tested_at,
        "brand": brand,
        "model": model,
        "performed_by": performed_by,
        "assessment": assessment,
        "certified": certified,
        "energy_current_kwh": energy_current_kwh,
        "energy_new_kwh": energy_new_kwh,
        "wltp_current_km": wltp_current_km,
        "wltp_new_km": wltp_new_km,
        "raw_soh_matches": raw_soh_matches,
    }
