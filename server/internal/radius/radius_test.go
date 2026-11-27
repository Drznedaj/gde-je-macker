package radius

import (
	"strings"
	"testing"
)

// Synthetic config with distinct, easy numbers.
const synth = `
version: 1
clamp_m: { min: 100, max: 4000 }
stage2_after_hours: 24
base:
  cat_indoor_only: { stage1_m: 200, stage2_m: 400 }
  cat_outdoor: { stage1_m: 500, stage2_m: 1000 }
  dog: { stage1_m: 800, stage2_m: 1600 }
multipliers:
  cat_intact_male: { value: 2 }
  dog_rural: { value: 3 }
  cat_rural: { value: 1.5 }
  dog_sex: { value: 1.1 }
`

func mustParse(t *testing.T, s string) *Config {
	t.Helper()
	c, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCalculate(t *testing.T) {
	c := mustParse(t, synth)
	// Neutral cat/dog inputs: female, neutered, urban.
	tests := []struct {
		name   string
		in     Input
		s1, s2 float64
	}{
		{"cat indoor base", Input{SpeciesCat, OutsideNo, SexFemale, NeuteredYes, DensityUrban}, 200, 400},
		{"cat outdoor base", Input{SpeciesCat, OutsideYes, SexFemale, NeuteredYes, DensityUrban}, 500, 1000},
		{"dog base", Input{SpeciesDog, OutsideYes, SexFemale, NeuteredYes, DensityUrban}, 800, 1600},
		{"cat intact male", Input{SpeciesCat, OutsideYes, SexMale, NeuteredNo, DensityUrban}, 1000, 2000},
		{"cat neutered male no multiplier", Input{SpeciesCat, OutsideYes, SexMale, NeuteredYes, DensityUrban}, 500, 1000},
		{"intact female cat no multiplier", Input{SpeciesCat, OutsideYes, SexFemale, NeuteredNo, DensityUrban}, 500, 1000},
		{"cat rural", Input{SpeciesCat, OutsideYes, SexFemale, NeuteredYes, DensityRural}, 750, 1500},
		{"cat intact male rural", Input{SpeciesCat, OutsideYes, SexMale, NeuteredNo, DensityRural}, 1500, 3000},
		{"dog rural", Input{SpeciesDog, OutsideYes, SexFemale, NeuteredYes, DensityRural}, 2400, 4000},
		{"dog male sex multiplier", Input{SpeciesDog, OutsideYes, SexMale, NeuteredYes, DensityUrban}, 880, 1760},
		{"dog neutered flag ignored", Input{SpeciesDog, OutsideYes, SexFemale, NeuteredNo, DensityUrban}, 800, 1600},
		// Clamping.
		{"clamp max", Input{SpeciesDog, OutsideYes, SexMale, NeuteredYes, DensityRural}, 2640, 4000},
		// Unknown fallbacks.
		{"cat unknown outside -> outdoor", Input{SpeciesCat, OutsideUnknown, SexFemale, NeuteredYes, DensityUrban}, 500, 1000},
		{"cat unknown sex -> intact male", Input{SpeciesCat, OutsideYes, SexUnknown, NeuteredNo, DensityUrban}, 1000, 2000},
		{"cat unknown neutered -> intact male", Input{SpeciesCat, OutsideYes, SexMale, NeuteredUnknown, DensityUrban}, 1000, 2000},
		{"cat unknown density -> rural", Input{SpeciesCat, OutsideYes, SexFemale, NeuteredYes, DensityUnknown}, 750, 1500},
		{"dog unknown density -> rural", Input{SpeciesDog, OutsideYes, SexFemale, NeuteredYes, DensityUnknown}, 2400, 4000},
		{"dog unknown sex -> dog_sex", Input{SpeciesDog, OutsideYes, SexUnknown, NeuteredYes, DensityUrban}, 880, 1760},
		{"unknown species -> larger per stage", Input{SpeciesUnknown, OutsideYes, SexFemale, NeuteredYes, DensityUrban}, 800, 1600},
		{"unknown species, cat larger", Input{SpeciesUnknown, OutsideYes, SexMale, NeuteredNo, DensityUrban}, 1000, 2000},
		{"empty input -> all unknown", Input{}, 4000 * 0, 4000},
	}
	// Last case: dog rural+sex = 800*3*1.1=2640 / 1600*3.3 clamped 4000.
	tests[len(tests)-1].s1 = 2640
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Calculate(tc.in)
			if got.Stage1M != tc.s1 || got.Stage2M != tc.s2 {
				t.Fatalf("got %+v, want {%v %v}", got, tc.s1, tc.s2)
			}
		})
	}
}

func TestClampMin(t *testing.T) {
	c := mustParse(t, strings.Replace(synth, "cat_indoor_only: { stage1_m: 200", "cat_indoor_only: { stage1_m: 50", 1))
	got := c.Calculate(Input{SpeciesCat, OutsideNo, SexFemale, NeuteredYes, DensityUrban})
	if got.Stage1M != 100 {
		t.Fatalf("stage1 = %v, want clamped to 100", got.Stage1M)
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"min > max":       strings.Replace(synth, "min: 100, max: 4000", "min: 5000, max: 4000", 1),
		"missing clamp":   strings.Replace(synth, "clamp_m: { min: 100, max: 4000 }", "", 1),
		"missing dog":     strings.Replace(synth, "  dog: { stage1_m: 800, stage2_m: 1600 }\n", "", 1),
		"missing mult":    strings.Replace(synth, "  dog_sex: { value: 1.1 }\n", "", 1),
		"stage2 < stage1": strings.Replace(synth, "stage2_m: 1600", "stage2_m: 100", 1),
		"not yaml":        "{{{",
	}
	for name, s := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(s)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadRealConfig(t *testing.T) {
	c, err := Load("../../../config/radius.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := c.Calculate(Input{SpeciesCat, OutsideNo, SexFemale, NeuteredYes, DensityUrban})
	if r.Stage1M < c.ClampM.Min || r.Stage2M > c.ClampM.Max || r.Stage1M > r.Stage2M {
		t.Fatalf("implausible result %+v", r)
	}
}
