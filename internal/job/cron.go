package job

import (
	"fmt"
	errs "go-scheduler/internal/error"
	"strconv"
	"strings"
	"time"
)

// CronSchedule represents a parsed cron expression.
// It defines allowed scheduling times using slices of integers for each cron field (minutes, hours, days, months, weekdays).
type CronSchedule struct {
	Minutes  []int // Allowed minute values (0-59)
	Hours    []int // Allowed hour values (0-23)
	Days     []int // Allowed day-of-month values (1-31)
	Months   []int // Allowed month values (1-12)
	Weekdays []int // Allowed weekday values (0-6, where 0 = Sunday)
}

// parseField parses an individual cron expression field (e.g., minutes, hours)
// and returns a slice of integers representing the allowed values.
//
// Supported syntax:
//   - "*": wildcard, matches all values within the range.
//   - "*/N": step values, matches every Nth value in the range.
//   - "X-Y": range values, matches all values between X and Y inclusive.
//   - "X,Y,Z": specific values, matches only listed values.
//
// Parameters:
//   - field: Cron field string to parse (e.g., "*/5", "1,2,3", "10-15").
//   - min, max: Allowed range for this cron field.
//
// Returns:
//   - A slice of parsed integer values.
//   - An error if parsing fails due to invalid syntax or values out of range.
func parseField(field string, min int, max int) ([]int, error) {
	var values []int

	if field == "*" {
		for i := min; i <= max; i++ {
			values = append(values, i)
		}
		return values, nil
	}

	parts := strings.Split(field, ",")
	for _, part := range parts {
		if strings.Contains(part, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step in field: %s", part)
			}
			for i := min; i <= max; i += step {
				values = append(values, i)
			}
		} else if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 != nil || err2 != nil || start > end || start < min || end > max {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			for i := start; i <= end; i++ {
				values = append(values, i)
			}
		} else {
			val, err := strconv.Atoi(part)
			if err != nil || val < min || val > max {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			values = append(values, val)
		}
	}

	return values, nil
}

// ParseCron parses a cron expression string and returns a CronSchedule instance.
// The cron expression must contain exactly five fields separated by spaces:
//
//	Minutes (0-59), Hours (0-23), Day-of-month (1-31), Month (1-12), Weekday (0-6, Sunday = 0).
//
// Parameters:
//   - cronExpr: The cron expression string (e.g., "*/5 * * * *").
//
// Returns:
//   - A pointer to a CronSchedule with parsed fields.
//   - An error if the cron expression syntax is invalid.
func ParseCron(cronExpr string) (*CronSchedule, error) {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return nil, errs.New(errs.ErrInvalidCronExpression, cronExpr)
	}

	minutes, err := parseField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minutes field error: %w", err)
	}
	hours, err := parseField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hours field error: %w", err)
	}
	days, err := parseField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("days field error: %w", err)
	}
	months, err := parseField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("months field error: %w", err)
	}
	weekdays, err := parseField(parts[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("weekdays field error: %w", err)
	}

	return &CronSchedule{
		Minutes:  minutes,
		Hours:    hours,
		Days:     days,
		Months:   months,
		Weekdays: weekdays,
	}, nil
}

// NextRun calculates and returns the next scheduled execution time based on the cron schedule.
// It efficiently skips unsuitable dates and times to find the next match, looking ahead up to one year.
//
// Returns:
//   - The next scheduled execution time (time.Time).
//   - Zero-value time if no valid execution time is found within a year (unlikely scenario).
func (cs *CronSchedule) NextRun() time.Time {
	now := time.Now().Truncate(time.Minute).Add(time.Minute)

	for i := 0; i < 365*24*60; i++ { // Safety limit: search up to 1 year ahead
		if !contains(cs.Months, int(now.Month())) {
			now = now.AddDate(0, 1, -now.Day()+1).Truncate(24 * time.Hour)
			continue
		}
		if !contains(cs.Days, now.Day()) {
			now = now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
			continue
		}
		if !contains(cs.Hours, now.Hour()) {
			now = now.Add(time.Hour - time.Duration(now.Minute())*time.Minute).Truncate(time.Hour)
			continue
		}
		if !contains(cs.Minutes, now.Minute()) {
			now = now.Add(time.Minute)
			continue
		}
		if !contains(cs.Weekdays, int(now.Weekday())) {
			now = now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
			continue
		}

		return now
	}

	// Should never happen; provided as a safeguard.
	return time.Time{}
}

// matches determines if a given time satisfies the cron schedule conditions.
//
// Parameters:
//   - t: Time value to evaluate.
//
// Returns:
//   - true if the provided time matches all cron fields; false otherwise.
func (cs *CronSchedule) matches(t time.Time) bool {
	return contains(cs.Minutes, t.Minute()) &&
		contains(cs.Hours, t.Hour()) &&
		contains(cs.Days, t.Day()) &&
		contains(cs.Months, int(t.Month())) &&
		contains(cs.Weekdays, int(t.Weekday()))
}

// contains checks whether a slice of integers contains a specified integer value.
//
// Parameters:
//   - arr: Slice of integers.
//   - val: Integer to search for.
//
// Returns:
//   - true if the integer is found in the slice; false otherwise.
func contains(arr []int, val int) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}
