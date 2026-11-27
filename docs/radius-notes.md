# Radius notes: classifying a point as urban or rural

Status: proposal only, nothing implemented. Sizes and licences below are from memory and were not re-checked (no web access during the audit); verify before deciding.

## Why

`config/radius.yaml` has `dog_rural` (1.5, weak) and `cat_rural` (1.0, weak). Applying them needs a rule that says whether a post location is urban or rural. It must not need user accounts or stored device locations, so it runs on the post's coarse location on the server, or on a geohash cell.

## Options

1. **GHSL SMOD (Degree of Urbanisation grid), JRC.** Global 1 km grid, classes from very low density rural to urban centre. Licence: CC BY 4.0 (JRC open data). The Europe or Serbia clip is small (a few MB as GeoTIFF); the global file is far larger. Consistent method, easy to cite.
2. **OSM landuse and place tags.** Extract of Serbia from Geofabrik is roughly 200-300 MB (PBF). Licence: ODbL (attribution and share-alike on derived databases). Finer than 1 km but needs heavy processing and tag quality varies.
3. **Own density from the post table.** Not viable at launch, no data.
4. **Settlement polygons from Serbian statistics or OSM `place=city|town|village`.** Small, but boundaries are inconsistent.

## Recommendation (simplest)

Option 1, precomputed offline: a script in `data/` reads the GHSL SMOD raster for Serbia once and writes a small lookup table keyed by geohash cell (precision matching the alert cells, about 5 characters) with a single class: `urban` or `rural`. Cell class = class of the cell centre pixel (or majority if cheap). The server loads this file at start, so there is no runtime dependency, no raster library in Go, and no per-request cost. Estimated size for Serbia at precision 5: tens of thousands of cells, well under 1 MB. Attribution: JRC GHSL, CC BY 4.0, noted in the app credits.

Fallback if SMOD proves too coarse: OSM `landuse=residential|commercial|industrial` plus `place` tags from the Geofabrik extract, processed with the same offline script and output format.

## Open items

- Pick a threshold: treat SMOD classes urban centre and urban cluster as urban, everything else rural.
- Check GHSL release, resolution and exact licence text before use.
- Validate against a handful of known Serbian locations (Belgrade, Nis, a village, a mountain area).
- No Serbian lost-pet distance data was found, so the multipliers stay weak regardless of the classifier.
