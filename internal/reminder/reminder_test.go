package reminder

import "testing"

func TestParseTime(t *testing.T) {
	hour, minute, err := parseTime("18:30")
	if err != nil || hour != 18 || minute != 30 {
		t.Fatalf("got %d:%d, %v", hour, minute, err)
	}
	if _, _, err := parseTime("25:00"); err == nil {
		t.Fatal("expected invalid hour")
	}
}

func TestParseTimeRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "9", "09:60", "-1:00", "24:00", "noon"} {
		if _, _, err := parseTime(value); err == nil {
			t.Errorf("parseTime(%q) unexpectedly succeeded", value)
		}
	}
}
