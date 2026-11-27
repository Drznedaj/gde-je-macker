// Package radius turns a post's attributes into alert radii (stage 1 and
// stage 2, in metres) using the table in config/radius.yaml.
package radius

import (
	"fmt"
	"math"
	"os"

	"gopkg.in/yaml.v3"
)

// Base is one base-case row of the table.
type Base struct {
	Stage1M    float64 `yaml:"stage1_m"`
	Stage2M    float64 `yaml:"stage2_m"`
	Confidence string  `yaml:"confidence"`
	Evidence   string  `yaml:"evidence"`
}

// Multiplier is one multiplier row of the table.
type Multiplier struct {
	Value      float64 `yaml:"value"`
	Confidence string  `yaml:"confidence"`
	Evidence   string  `yaml:"evidence"`
}

// Config mirrors config/radius.yaml.
type Config struct {
	Version int `yaml:"version"`
	ClampM  struct {
		Min float64 `yaml:"min"`
		Max float64 `yaml:"max"`
	} `yaml:"clamp_m"`
	Stage2AfterHours int `yaml:"stage2_after_hours"`
	Base             struct {
		CatIndoorOnly Base `yaml:"cat_indoor_only"`
		CatOutdoor    Base `yaml:"cat_outdoor"`
		Dog           Base `yaml:"dog"`
	} `yaml:"base"`
	Multipliers struct {
		CatIntactMale Multiplier `yaml:"cat_intact_male"`
		DogRural      Multiplier `yaml:"dog_rural"`
		CatRural      Multiplier `yaml:"cat_rural"`
		DogSex        Multiplier `yaml:"dog_sex"`
	} `yaml:"multipliers"`
}

// Parse decodes and validates a radius table. Unknown keys and missing
// required keys are errors.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("radius: parse: %w", err)
	}
	// Presence check via a generic decode (zero values are otherwise ambiguous).
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("radius: parse: %w", err)
	}
	if err := requireKeys(raw); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Load reads and parses a radius table from a file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("radius: read: %w", err)
	}
	return Parse(data)
}

func requireKeys(raw map[string]any) error {
	sub := func(m map[string]any, k string) (map[string]any, error) {
		v, ok := m[k].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("radius: missing or invalid key %q", k)
		}
		return v, nil
	}
	for _, k := range []string{"version", "stage2_after_hours"} {
		if _, ok := raw[k]; !ok {
			return fmt.Errorf("radius: missing key %q", k)
		}
	}
	clamp, err := sub(raw, "clamp_m")
	if err != nil {
		return err
	}
	for _, k := range []string{"min", "max"} {
		if _, ok := clamp[k]; !ok {
			return fmt.Errorf("radius: missing key clamp_m.%s", k)
		}
	}
	base, err := sub(raw, "base")
	if err != nil {
		return err
	}
	for _, name := range []string{"cat_indoor_only", "cat_outdoor", "dog"} {
		row, err := sub(base, name)
		if err != nil {
			return fmt.Errorf("base.%s: %w", name, err)
		}
		for _, k := range []string{"stage1_m", "stage2_m"} {
			if _, ok := row[k]; !ok {
				return fmt.Errorf("radius: missing key base.%s.%s", name, k)
			}
		}
	}
	mult, err := sub(raw, "multipliers")
	if err != nil {
		return err
	}
	for _, name := range []string{"cat_intact_male", "dog_rural", "cat_rural", "dog_sex"} {
		row, err := sub(mult, name)
		if err != nil {
			return fmt.Errorf("multipliers.%s: %w", name, err)
		}
		if _, ok := row["value"]; !ok {
			return fmt.Errorf("radius: missing key multipliers.%s.value", name)
		}
	}
	return nil
}

func (c *Config) validate() error {
	if c.ClampM.Min <= 0 || c.ClampM.Min > c.ClampM.Max {
		return fmt.Errorf("radius: clamp_m must satisfy 0 < min <= max (got %v, %v)", c.ClampM.Min, c.ClampM.Max)
	}
	for name, b := range map[string]Base{
		"cat_indoor_only": c.Base.CatIndoorOnly,
		"cat_outdoor":     c.Base.CatOutdoor,
		"dog":             c.Base.Dog,
	} {
		if b.Stage1M <= 0 || b.Stage2M < b.Stage1M {
			return fmt.Errorf("radius: base.%s needs 0 < stage1_m <= stage2_m", name)
		}
	}
	for name, m := range map[string]Multiplier{
		"cat_intact_male": c.Multipliers.CatIntactMale,
		"dog_rural":       c.Multipliers.DogRural,
		"cat_rural":       c.Multipliers.CatRural,
		"dog_sex":         c.Multipliers.DogSex,
	} {
		if m.Value <= 0 {
			return fmt.Errorf("radius: multipliers.%s.value must be > 0", name)
		}
	}
	return nil
}

// Species of the pet.
type Species string

// GoesOutside says whether the pet has outdoor access.
type GoesOutside string

// Sex of the pet.
type Sex string

// Neutered says whether the pet is neutered.
type Neutered string

// Density is the density class of the place where the pet was lost.
type Density string

const (
	SpeciesUnknown Species = "unknown"
	SpeciesCat     Species = "cat"
	SpeciesDog     Species = "dog"

	OutsideUnknown GoesOutside = "unknown"
	OutsideYes     GoesOutside = "yes"
	OutsideNo      GoesOutside = "no"

	SexUnknown Sex = "unknown"
	SexMale    Sex = "male"
	SexFemale  Sex = "female"

	NeuteredUnknown Neutered = "unknown"
	NeuteredYes     Neutered = "yes"
	NeuteredNo      Neutered = "no"

	DensityUnknown Density = "unknown"
	DensityUrban   Density = "urban"
	DensityRural   Density = "rural"
)

// Input is what the radius depends on. Any unrecognised or empty value is
// treated as "unknown".
type Input struct {
	Species     Species
	GoesOutside GoesOutside
	Sex         Sex
	Neutered    Neutered
	Density     Density
}

// Result holds the radii in metres, already clamped.
type Result struct {
	Stage1M float64
	Stage2M float64
}

// Calculate returns the stage 1 and stage 2 radii for in.
//
// Fallback rules for unknown values (always the larger-radius, i.e.
// conservative, case, so an alert is never narrower than the data supports):
//   - species unknown (or unrecognised): compute the cat and the dog result
//     with the other fields as given and take the larger value per stage.
//   - cat, goes_outside unknown: cat_outdoor (not indoor-only).
//   - cat, sex unknown or neutered unknown: cat_intact_male multiplier applies
//     (it applies when sex is male-or-unknown AND neutered is no-or-unknown).
//   - dog, sex unknown: dog_sex multiplier applies, as it does for males
//     (the table does not say which sex it targets; the male reading is used).
//   - density unknown: the rural multiplier applies (cat_rural / dog_rural).
//
// Results are multiplied, clamped to clamp_m and rounded to whole metres.
func (c *Config) Calculate(in Input) Result {
	switch in.Species {
	case SpeciesCat:
		return c.cat(in)
	case SpeciesDog:
		return c.dog(in)
	default:
		a, b := c.cat(in), c.dog(in)
		return Result{Stage1M: math.Max(a.Stage1M, b.Stage1M), Stage2M: math.Max(a.Stage2M, b.Stage2M)}
	}
}

func (c *Config) cat(in Input) Result {
	base := c.Base.CatOutdoor // yes and unknown
	if in.GoesOutside == OutsideNo {
		base = c.Base.CatIndoorOnly
	}
	m := 1.0
	if in.Sex != SexFemale && in.Neutered != NeuteredYes {
		m *= c.Multipliers.CatIntactMale.Value
	}
	if in.Density != DensityUrban {
		m *= c.Multipliers.CatRural.Value
	}
	return c.scale(base, m)
}

func (c *Config) dog(in Input) Result {
	m := 1.0
	if in.Density != DensityUrban {
		m *= c.Multipliers.DogRural.Value
	}
	if in.Sex != SexFemale {
		m *= c.Multipliers.DogSex.Value
	}
	return c.scale(c.Base.Dog, m)
}

func (c *Config) scale(b Base, m float64) Result {
	clamp := func(v float64) float64 {
		return math.Round(math.Min(math.Max(v*m, c.ClampM.Min), c.ClampM.Max))
	}
	return Result{Stage1M: clamp(b.Stage1M), Stage2M: clamp(b.Stage2M)}
}
