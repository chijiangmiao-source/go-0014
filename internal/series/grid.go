package series

import "steam-sterilization-thermal-validation/internal/domain"

type Point struct {
	AtNanos int64
	Value   float64
}

func FixedGrid(startNanos, endNanos, stepNanos int64) ([]int64, error) {
	if endNanos < startNanos || stepNanos <= 0 {
		return nil, domain.NewInputError(domain.CodeInvalidInput, "invalid grid bounds or step")
	}
	var grid []int64
	for t := startNanos; t <= endNanos; t += stepNanos {
		grid = append(grid, t)
		if endNanos-t < stepNanos {
			break
		}
	}
	if grid[len(grid)-1] != endNanos {
		grid = append(grid, endNanos)
	}
	return grid, nil
}

func Interpolate(points []Point, atNanos, maxGapNanos int64) (float64, error) {
	if len(points) == 0 || atNanos < points[0].AtNanos || atNanos > points[len(points)-1].AtNanos {
		return 0, domain.NewInputError(domain.CodeInvalidInput, "interpolation cannot extrapolate")
	}
	for i := 0; i < len(points); i++ {
		if points[i].AtNanos == atNanos {
			return points[i].Value, nil
		}
		if i+1 < len(points) && points[i].AtNanos < atNanos && atNanos < points[i+1].AtNanos {
			gap := points[i+1].AtNanos - points[i].AtNanos
			if gap > maxGapNanos {
				return 0, domain.NewInputError(domain.CodeInvalidInput, "sample gap exceeds max gap")
			}
			ratio := float64(atNanos-points[i].AtNanos) / float64(gap)
			return points[i].Value + ratio*(points[i+1].Value-points[i].Value), nil
		}
	}
	return 0, domain.NewInputError(domain.CodeInvalidInput, "interpolation point not covered")
}
