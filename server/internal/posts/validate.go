package posts

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits mirror the CHECK constraints in migrations/0001_init.sql and the
// schema in docs/api/openapi.yaml. They are enforced here first so clients
// get a clear 422 instead of a database error.
const (
	maxName        = 80
	maxDescription = 2000
	minContact     = 3
	maxContact     = 200
	maxPhotos      = 3

	// Rough bounding box of Serbia (includes small slices of neighbours).
	serbiaMinLat, serbiaMaxLat = 42.2, 46.2
	serbiaMinLon, serbiaMaxLon = 18.8, 23.1
)

var (
	kinds     = map[string]bool{"lost": true, "found": true}
	speciesOK = map[string]bool{"dog": true, "cat": true}
	sexes     = map[string]bool{"male": true, "female": true, "unknown": true}
	tri       = map[string]bool{"yes": true, "no": true, "unknown": true}

	uuidRe  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	tokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`) // base64url of 32 bytes; never contains "/" or "%"
)

// fieldErrors maps a field name to a human-readable problem.
type fieldErrors map[string]string

// validateContact is the single place that decides what a contact value may
// be. Story 011 (ADR 0003) makes contact optional: change only this function
// (return "" when c is nil or empty).
func validateContact(c *string) (value string, problem string) {
	if c == nil {
		return "", "is required"
	}
	v := strings.TrimSpace(*c)
	if n := utf8.RuneCountInString(v); n < minContact || n > maxContact {
		return "", fmt.Sprintf("must be %d-%d characters", minContact, maxContact)
	}
	return v, ""
}

func validateText(field, v string, max int, errs fieldErrors) {
	if utf8.RuneCountInString(v) > max {
		errs[field] = fmt.Sprintf("must be at most %d characters", max)
	}
}

func validateEnum(field string, v string, allowed map[string]bool, errs fieldErrors) {
	if !allowed[v] {
		errs[field] = "invalid value"
	}
}

func inSerbia(lat, lon float64) bool {
	return lat >= serbiaMinLat && lat <= serbiaMaxLat && lon >= serbiaMinLon && lon <= serbiaMaxLon
}

// validatePhotoIDs checks count, format and duplicates, and returns the ids.
func validatePhotoIDs(ids []string, errs fieldErrors) []string {
	if len(ids) > maxPhotos {
		errs["photo_ids"] = fmt.Sprintf("at most %d photos", maxPhotos)
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.ToLower(id)
		if !uuidRe.MatchString(id) {
			errs["photo_ids"] = "must contain valid photo ids"
			return nil
		}
		if seen[id] {
			errs["photo_ids"] = "duplicate photo id"
			return nil
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

type createInput struct {
	Kind        string     `json:"kind"`
	Species     string     `json:"species"`
	Sex         string     `json:"sex"`
	Neutered    string     `json:"neutered"`
	GoesOutside string     `json:"goes_outside"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Point       *pointJSON `json:"last_seen_point"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
	Contact     *string    `json:"contact"`
	PhotoIDs    []string   `json:"photo_ids"`
}

type pointJSON struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

// validated is a createInput that passed validation, with defaults applied.
type validated struct {
	createInput
	lat, lon float64
	contact  string
}

func (in createInput) validate(now time.Time) (validated, fieldErrors) {
	errs := fieldErrors{}
	v := validated{createInput: in}
	validateEnum("kind", in.Kind, kinds, errs)
	validateEnum("species", in.Species, speciesOK, errs)
	for _, d := range []struct {
		field string
		p     *string
		allow map[string]bool
	}{{"sex", &v.Sex, sexes}, {"neutered", &v.Neutered, tri}, {"goes_outside", &v.GoesOutside, tri}} {
		if *d.p == "" {
			*d.p = "unknown"
		}
		validateEnum(d.field, *d.p, d.allow, errs)
	}
	v.Name = strings.TrimSpace(in.Name)
	v.Description = strings.TrimSpace(in.Description)
	validateText("name", v.Name, maxName, errs)
	validateText("description", v.Description, maxDescription, errs)

	switch {
	case in.Point == nil || in.Point.Lat == nil || in.Point.Lon == nil:
		errs["last_seen_point"] = "lat and lon are required"
	case !inSerbia(*in.Point.Lat, *in.Point.Lon):
		errs["last_seen_point"] = "must be inside Serbia"
	default:
		v.lat, v.lon = *in.Point.Lat, *in.Point.Lon
	}
	switch {
	case in.LastSeenAt == nil:
		errs["last_seen_at"] = "is required"
	case in.LastSeenAt.After(now.Add(5 * time.Minute)):
		errs["last_seen_at"] = "must not be in the future"
	case in.LastSeenAt.Before(now.AddDate(-1, 0, 0)):
		errs["last_seen_at"] = "is too far in the past"
	}
	if c, problem := validateContact(in.Contact); problem != "" {
		errs["contact"] = problem
	} else {
		v.contact = c
	}
	v.PhotoIDs = validatePhotoIDs(in.PhotoIDs, errs)
	return v, errs
}

type updateInput struct {
	Description *string   `json:"description"`
	Contact     *string   `json:"contact"`
	PhotoIDs    *[]string `json:"photo_ids"`
}
