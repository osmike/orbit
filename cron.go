package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSchedule represents a parsed cron expression.
// It stores allowed values for each time unit (minutes, hours, etc.).
type CronSchedule struct {
	Minutes  []int // Allowed minute values (0-59)
	Hours    []int // Allowed hour values (0-23)
	Days     []int // Allowed day-of-month values (1-31)
	Months   []int // Allowed month values (1-12)
	Weekdays []int // Allowed weekday values (0-6, where 0 = Sunday)
}

// parseField parses a cron field and returns a list of allowed values.
//
// The function supports:
// - "*" (wildcard) -> includes all values within the range
// - "*/N" (step) -> includes values at intervals of N
// - "X-Y" (range) -> includes all values from X to Y
// - "X,Y,Z" (list) -> includes specific values
//
// Parameters:
// - field: The cron field string (e.g., "*/5", "1,2,3", "10-15").
// - min, max: The allowed range for this field.
//
// Returns:
// - A slice of integers representing the parsed values.
// - An error if the input is invalid.
func parseField(field string, min int, max int) ([]int, error) {
	var values []int

	// Wildcard: selects all possible values in the range
	if field == "*" {
		for i := min; i <= max; i++ {
			values = append(values, i)
		}
		return values, nil
	}

	// Splitting multiple values (comma-separated)
	parts := strings.Split(field, ",")
	for _, part := range parts {
		// Step values (*/N)
		if strings.Contains(part, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil {
				return nil, fmt.Errorf("invalid step in field: %s", part)
			}
			for i := min; i <= max; i += step {
				values = append(values, i)
			}
		} else if strings.Contains(part, "-") { // Range values (X-Y)
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 != nil || err2 != nil || start > end {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			for i := start; i <= end; i++ {
				values = append(values, i)
			}
		} else { // Single numeric values (X)
			val, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			values = append(values, val)
		}
	}

	return values, nil
}

// ParseCron parses a cron expression into a CronSchedule.
//
// The cron expression must consist of exactly five fields:
// - Minutes (0-59)
// - Hours (0-23)
// - Days of the month (1-31)
// - Months (1-12)
// - Weekdays (0-6, where 0 = Sunday)
//
// Parameters:
// - cronExpr: A cron expression string (e.g., "*/5 * * * *").
//
// Returns:
// - A CronSchedule struct with parsed values.
// - An error if the input cron expression is invalid.
func ParseCron(cronExpr string) (*CronSchedule, error) {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid cron expression: %s", cronExpr)
	}

	minutes, err := parseField(parts[0], 0, 59)
	if err != nil {
		return nil, err
	}
	hours, err := parseField(parts[1], 0, 23)
	if err != nil {
		return nil, err
	}
	days, err := parseField(parts[2], 1, 31)
	if err != nil {
		return nil, err
	}
	months, err := parseField(parts[3], 1, 12)
	if err != nil {
		return nil, err
	}
	weekdays, err := parseField(parts[4], 0, 6)
	if err != nil {
		return nil, err
	}

	return &CronSchedule{
		Minutes:  minutes,
		Hours:    hours,
		Days:     days,
		Months:   months,
		Weekdays: weekdays,
	}, nil
}

// NextRun calculates the duration until the next execution time based on the cron schedule.
//
// It iterates minute-by-minute into the future (up to one year) to find the next valid execution time.
//
// Returns:
// - The duration until the next valid run time.
func (cs *CronSchedule) NextRun() time.Duration {
	now := time.Now()

	// Iterate minute-by-minute, searching for a valid match
	for i := 0; i < 365*24*60; i++ {
		next := now.Add(time.Duration(i) * time.Minute)
		if cs.matches(next) {
			return time.Until(next)
		}
	}
	return 0
}

// matches checks if a given time matches the parsed cron schedule.
//
// Parameters:
// - t: The time to check.
//
// Returns:
// - True if the time matches the schedule, otherwise false.
func (cs *CronSchedule) matches(t time.Time) bool {
	return contains(cs.Minutes, t.Minute()) &&
		contains(cs.Hours, t.Hour()) &&
		contains(cs.Days, t.Day()) &&
		contains(cs.Months, int(t.Month())) &&
		contains(cs.Weekdays, int(t.Weekday()))
}

// contains checks if an integer is present in a slice.
//
// Parameters:
// - arr: The slice of integers.
// - val: The value to check.
//
// Returns:
// - True if the value is found, otherwise false.
func contains(arr []int, val int) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}
