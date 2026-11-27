package radius

import "testing"

// Spot checks against the values in config/radius.yaml (version 0).
func TestRealConfigSpotValues(t *testing.T) {
	c, err := Load("../../../config/radius.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		in     Input
		s1, s2 float64
	}{
		{"indoor cat female neutered urban", Input{SpeciesCat, OutsideNo, SexFemale, NeuteredYes, DensityUrban}, 300, 500},
		{"outdoor cat intact male urban", Input{SpeciesCat, OutsideYes, SexMale, NeuteredNo, DensityUrban}, 625, 2000},
		{"outdoor cat female neutered rural", Input{SpeciesCat, OutsideYes, SexFemale, NeuteredYes, DensityRural}, 500, 1600},
		{"dog urban", Input{SpeciesDog, OutsideYes, SexFemale, NeuteredYes, DensityUrban}, 800, 1600},
		{"dog rural", Input{SpeciesDog, OutsideYes, SexMale, NeuteredNo, DensityRural}, 1200, 2400},
		{"all unknown", Input{}, 1200, 2400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Calculate(tc.in)
			if got.Stage1M != tc.s1 || got.Stage2M != tc.s2 {
				t.Fatalf("got %+v, want {%v %v}", got, tc.s1, tc.s2)
			}
		})
	}
}
