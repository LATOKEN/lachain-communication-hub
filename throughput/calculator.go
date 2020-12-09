package throughput

import "time"

type Calculator struct {
	lastReset    time.Time
	threshold    time.Duration
	measurements int32
	sum          float64
	report       func(float64, int32, time.Duration)
}

func New(threshold time.Duration, report func(float64, int32, time.Duration)) *Calculator {
	calc := new(Calculator)
	calc.lastReset = time.Now()
	calc.measurements = 0
	calc.sum = 0
	calc.threshold = threshold
	calc.report = report
	return calc
}

func (calculator *Calculator) AddMeasurement(measurement float64) {
	calculator.sum += measurement
	calculator.measurements += 1
	t := time.Now()
	if t.Sub(calculator.lastReset) > calculator.threshold {
		sum, measurements, duration := calculator.reset(t)
		calculator.report(sum, measurements, duration)
	}
}

func (calculator *Calculator) reset(now time.Time) (float64, int32, time.Duration) {
	sum := calculator.sum
	measurements := calculator.measurements
	duration := now.Sub(calculator.lastReset)
	calculator.sum = 0
	calculator.measurements = 0
	calculator.lastReset = now
	return sum, measurements, duration
}
