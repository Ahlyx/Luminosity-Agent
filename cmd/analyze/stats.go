package main

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/stat"
)

// concentrationIndex returns what percentage of the total the top-N values hold.
// Values are expected to be non-negative. Returns 0 for empty input or topN <= 0.
func concentrationIndex(values []float64, topN int) float64 {
	if len(values) == 0 || topN <= 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] > sorted[j] })

	var total float64
	for _, v := range values {
		total += v
	}
	if total == 0 {
		return 0
	}

	n := min(topN, len(sorted))
	var topSum float64
	for i := range n {
		topSum += sorted[i]
	}
	return topSum / total * 100.0
}

// anomalyThreshold returns mean + stddevs * standard-deviation for the given slice.
func anomalyThreshold(values []float64, stddevs float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mu := stat.Mean(values, nil)
	sigma := stat.StdDev(values, nil)
	return mu + stddevs*sigma
}

// trendDirection returns "increasing", "decreasing", or "stable" based on the
// sign of the linear-regression slope of (xvals, yvals), normalised by the
// mean of yvals (threshold ±5 % per unit-x).
func trendDirection(xvals, yvals []float64) string {
	if len(xvals) < 2 || len(xvals) != len(yvals) {
		return "stable"
	}
	_, slope := stat.LinearRegression(xvals, yvals, nil, false)
	mean := stat.Mean(yvals, nil)
	if mean == 0 {
		if slope > 0 {
			return "increasing"
		} else if slope < 0 {
			return "decreasing"
		}
		return "stable"
	}
	rel := slope / math.Abs(mean)
	const threshold = 0.05
	switch {
	case rel > threshold:
		return "increasing"
	case rel < -threshold:
		return "decreasing"
	default:
		return "stable"
	}
}

// hurstExponent estimates the Hurst exponent via rescaled-range (R/S) analysis.
// Returns 0.5 (random walk) if the series has fewer than 20 data points or if
// there are not enough lag windows to fit a line.
func hurstExponent(series []float64) float64 {
	if len(series) < 20 {
		return 0.5
	}

	var xs, ys []float64
	for lag := 10; lag <= len(series)/2; lag *= 2 {
		rs := rsStatistic(series[:lag])
		if rs > 0 {
			xs = append(xs, math.Log(float64(lag)))
			ys = append(ys, math.Log(rs))
		}
	}
	if len(xs) < 2 {
		return 0.5
	}

	_, h := stat.LinearRegression(xs, ys, nil, false)
	if h < 0 || h > 1 {
		return 0.5
	}
	return h
}

// rsStatistic computes the rescaled range (R/S) for a sub-series.
func rsStatistic(series []float64) float64 {
	n := len(series)
	if n < 2 {
		return 0
	}
	mean := stat.Mean(series, nil)

	cum := make([]float64, n)
	cum[0] = series[0] - mean
	for i := 1; i < n; i++ {
		cum[i] = cum[i-1] + (series[i] - mean)
	}

	lo, hi := cum[0], cum[0]
	for _, v := range cum {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	R := hi - lo
	S := stat.StdDev(series, nil)
	if S == 0 {
		return 0
	}
	return R / S
}
