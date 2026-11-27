#!/usr/bin/env python3
"""Deterministic calibration summary for config/radius.yaml.

Input: CSV with columns species, distance_m, class (e.g. cat_indoor_only,
cat_outdoor, dog). Only distances and classes are read; never coordinates.
Output: JSON with count and nearest-rank 50th/75th/90th percentiles per class.
Classes with fewer than MIN_N rows are reported as insufficient.
Usage: python3 -I data/calibrate_radius.py reunions.csv
"""
import csv
import json
import math
import sys

MIN_N = 100
PCTS = (50, 75, 90)


def nearest_rank(sorted_vals, p):
    k = max(1, math.ceil(p / 100 * len(sorted_vals)))
    return sorted_vals[k - 1]


def summarize(rows):
    groups = {}
    for r in rows:
        key = f"{r['species']}/{r['class']}"
        groups.setdefault(key, []).append(float(r["distance_m"]))
    out = {}
    for key in sorted(groups):
        vals = sorted(groups[key])
        if len(vals) < MIN_N:
            out[key] = {"n": len(vals), "status": "insufficient"}
        else:
            out[key] = {"n": len(vals), **{f"p{p}": nearest_rank(vals, p) for p in PCTS}}
    return out


def main(argv):
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    with open(argv[1], newline="") as f:
        rows = list(csv.DictReader(f))
    print(json.dumps(summarize(rows), indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
