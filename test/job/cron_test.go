package job_test

import (
	"github.com/osmike/orbit/internal/job"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseField_Wildcard(t *testing.T) {
	values, err := job.ParseField("*", 0, 5)
	assert.NoError(t, err)
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5}, values)
}

func TestParseField_Step(t *testing.T) {
	values, err := job.ParseField("*/2", 0, 6)
	assert.NoError(t, err)
	assert.Equal(t, []int{0, 2, 4, 6}, values)
}

func TestParseField_Range(t *testing.T) {
	values, err := job.ParseField("2-4", 0, 6)
	assert.NoError(t, err)
	assert.Equal(t, []int{2, 3, 4}, values)
}

func TestParseField_List(t *testing.T) {
	values, err := job.ParseField("1,3,5", 0, 6)
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 3, 5}, values)
}

func TestParseField_InvalidStep(t *testing.T) {
	_, err := job.ParseField("*/a", 0, 5)
	assert.Error(t, err)
}

func TestParseField_InvalidRange(t *testing.T) {
	_, err := job.ParseField("5-2", 0, 5)
	assert.Error(t, err)
}

func TestParseField_InvalidValue(t *testing.T) {
	_, err := job.ParseField("10", 0, 5)
	assert.Error(t, err)
}

func TestParseCron_Valid(t *testing.T) {
	cron, err := job.ParseCron("*/5 0 1 1 0")
	assert.NoError(t, err)
	assert.NotNil(t, cron)
	assert.Equal(t, []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55}, cron.Minutes)
	assert.Equal(t, []int{0}, cron.Hours)
	assert.Equal(t, []int{1}, cron.Days)
	assert.Equal(t, []int{1}, cron.Months)
	assert.Equal(t, []int{0}, cron.Weekdays)
}

func TestParseCron_InvalidFieldCount(t *testing.T) {
	_, err := job.ParseCron("*/5 0 1 1")
	assert.Error(t, err)
}

func TestCronSchedule_Matches(t *testing.T) {
	cs := &job.CronSchedule{
		Minutes:  []int{15},
		Hours:    []int{10},
		Days:     []int{5},
		Months:   []int{3},
		Weekdays: []int{3}, // Wednesday
	}

	tm := time.Date(2025, 3, 5, 10, 15, 0, 0, time.UTC)
	assert.True(t, cs.Matches(tm))

	tm2 := time.Date(2025, 3, 5, 10, 16, 0, 0, time.UTC)
	assert.False(t, cs.Matches(tm2))
}

func TestCronSchedule_NextRun(t *testing.T) {
	cs := &job.CronSchedule{
		Minutes:  []int{59},
		Hours:    []int{23},
		Days:     []int{31},
		Months:   []int{12},
		Weekdays: []int{3, 4, 5, 6}, // Thu, Fri, Sat, Sun
	}

	next := cs.NextRun(time.Now())
	assert.False(t, next.IsZero())
}
