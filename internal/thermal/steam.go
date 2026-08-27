package thermal

import "steam-sterilization-thermal-validation/internal/domain"

func ValidateSteamTable(table []domain.SteamPoint) error {
	if len(table) < 2 {
		return domain.NewInputError(domain.CodeInvalidInput, "steam table requires at least two points")
	}
	for i := 1; i < len(table); i++ {
		if table[i].PressureKPa <= table[i-1].PressureKPa {
			return domain.NewInputError(domain.CodeInvalidInput, "steam pressure must be strictly increasing")
		}
		if table[i].SaturatedC < table[i-1].SaturatedC {
			return domain.NewInputError(domain.CodeInvalidInput, "steam temperature must be monotonic")
		}
	}
	return nil
}

func SaturatedTemperature(table []domain.SteamPoint, pressureKPa float64) (float64, error) {
	if err := ValidateSteamTable(table); err != nil {
		return 0, err
	}
	if pressureKPa < table[0].PressureKPa || pressureKPa > table[len(table)-1].PressureKPa {
		return 0, domain.NewInputError(domain.CodeInvalidInput, "pressure outside frozen steam table")
	}
	for i := 0; i < len(table); i++ {
		if table[i].PressureKPa == pressureKPa {
			return table[i].SaturatedC, nil
		}
		if i+1 < len(table) && table[i].PressureKPa < pressureKPa && pressureKPa < table[i+1].PressureKPa {
			ratio := (pressureKPa - table[i].PressureKPa) / (table[i+1].PressureKPa - table[i].PressureKPa)
			return table[i].SaturatedC + ratio*(table[i+1].SaturatedC-table[i].SaturatedC), nil
		}
	}
	return 0, domain.NewInputError(domain.CodeInvalidInput, "pressure not covered by steam table")
}
