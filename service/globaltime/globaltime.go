package globaltime

import "time"

// DateFormat is how every date is written and read across the app
// Fixed-width UTC milliseconds keep close operations distinct and make text order chronological
const DateFormat = "2006-01-02T15:04:05.000Z"

// FixedTime represent a fixed moment in time. Set this variable to anything different from the default value for
// time.Time and the value will be returned in Now() function in place of the current time
var FixedTime time.Time

// Now returns the current time (time.Now()) if no FixedTime has been set. Otherwise, it returns FixedTime
// Use this in place of time.Now() to allow testing w/ custom time
func Now() time.Time {
	if FixedTime.After(time.Time{}) {
		return FixedTime
	}
	return time.Now()
}

// Since returns the time passed since the parameter tm.
func Since(tm time.Time) time.Duration {
	return Now().Sub(tm)
}

// Format writes t using DateFormat
// Always call this on a .UTC() time, since DateFormat's trailing "Z"
// is a literal character and not the Z07:00 verb: it does not convert the zone itself
func Format(t time.Time) string {
	return t.UTC().Format(DateFormat)
}

// Parse reverses Format: every date column is written with it,
// so every date column is read back with it
// A date that fails to parse means the column or the format drifted
// .UTC() normalizes the location, since DateFormat's trailing "Z" is a literal character and
// not the Z07:00 verb, so time.Parse alone would not guarantee a UTC-tagged result
func Parse(text string) (time.Time, error) {
	t, err := time.Parse(DateFormat, text)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
