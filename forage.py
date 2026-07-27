#!/usr/bin/env python3
"""Forage — discover package files, digests, and provenance from package indexes."""

import argparse
import json
import re
import sys
from html.parser import HTMLParser
from urllib.parse import urldefrag

import requests


class SimpleIndexParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.files = []
        self._current = None

    def handle_starttag(self, tag, attrs):
        if tag != "a":
            return
        attrs_dict = dict(attrs)
        href = attrs_dict.get("href", "")
        url, fragment = urldefrag(href)
        sha256 = ""
        if fragment.startswith("sha256="):
            sha256 = fragment[len("sha256="):]
        self._current = {
            "url": url,
            "sha256": sha256,
            "provenance_url": attrs_dict.get("data-provenance", None),
            "filename": "",
        }

    def handle_data(self, data):
        if self._current is not None:
            self._current["filename"] += data

    def handle_endtag(self, tag):
        if tag == "a" and self._current is not None:
            self._current["filename"] = self._current["filename"].strip()
            self.files.append(self._current)
            self._current = None


def normalize_name(name):
    return re.sub(r"[-_.]+", "-", name).lower()


def matches_version(filename, package, version):
    prefix = f"{normalize_name(package)}-{version}"
    normalized_filename = normalize_name(filename.split("-")[0]) + "-" + "-".join(filename.split("-")[1:])
    # After the version, expect '.' (sdist) or '-' (wheel build/python tag)
    if not normalized_filename.startswith(prefix):
        return False
    rest = normalized_filename[len(prefix):]
    return rest and rest[0] in (".", "-")


def fetch_simple_index(index_url, package):
    url = f"{index_url.rstrip('/')}/{package}/"
    resp = requests.get(url)
    if resp.status_code == 404:
        print(f"error: package '{package}' not found at {url}", file=sys.stderr)
        sys.exit(1)
    resp.raise_for_status()
    return resp.text


def fetch_provenance(provenance_url):
    try:
        resp = requests.get(provenance_url)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        return {"error": str(e)}


def main():
    parser = argparse.ArgumentParser(
        prog="forage",
        description="Discover package files, digests, and provenance from package indexes.",
    )
    parser.add_argument("package", help="Package name")
    parser.add_argument("version", help="Package version")
    parser.add_argument(
        "--index-url",
        default="https://pypi.org/simple/",
        help="PEP 503 simple index URL (default: https://pypi.org/simple/)",
    )
    parser.add_argument("--json", dest="output_json", action="store_true", help="Output JSON")
    parser.add_argument(
        "--fetch-provenance",
        action="store_true",
        help="Fetch and inline provenance attestation data",
    )
    args = parser.parse_args()

    html = fetch_simple_index(args.index_url, args.package)

    index_parser = SimpleIndexParser()
    index_parser.feed(html)

    matched = [f for f in index_parser.files if matches_version(f["filename"], args.package, args.version)]

    if not matched:
        print(
            f"error: no files found for {args.package}=={args.version}",
            file=sys.stderr,
        )
        sys.exit(1)

    if args.fetch_provenance:
        for entry in matched:
            if entry["provenance_url"]:
                entry["provenance"] = fetch_provenance(entry["provenance_url"])

    results = []
    for entry in matched:
        item = {"filename": entry["filename"], "sha256": entry["sha256"]}
        item["provenance_url"] = entry["provenance_url"]
        if "provenance" in entry:
            item["provenance"] = entry["provenance"]
        results.append(item)

    if args.output_json:
        output = {"package": args.package, "version": args.version, "files": results}
        print(json.dumps(output, indent=2))
    else:
        print(f"{args.package} {args.version}\n")
        for item in results:
            print(f"  {item['filename']}")
            print(f"    sha256: {item['sha256']}")
            prov = item.get("provenance_url") or "(none)"
            print(f"    provenance: {prov}")
            if "provenance" in item:
                print(f"    provenance data: (fetched, {len(json.dumps(item['provenance']))} bytes)")
            print()


if __name__ == "__main__":
    main()
