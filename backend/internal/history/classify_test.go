package history

import "testing"

func TestShouldPersist(t *testing.T) {
	cases := []struct {
		name     string
		position int64
		duration int64
		want     bool
	}{
		{name: "zero", position: 0, duration: 15000, want: false},
		{name: "too shallow", position: 2000, duration: 15000, want: false},
		{name: "three seconds", position: 3000, duration: 60000, want: true},
		{name: "twenty percent of short", position: 2000, duration: 8000, want: true},
		{name: "unknown duration below 3s", position: 2000, duration: 0, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldPersist(tc.position, tc.duration); got != tc.want {
				t.Fatalf("ShouldPersist(%d,%d)=%v, want %v", tc.position, tc.duration, got, tc.want)
			}
		})
	}
}

func TestIsCompleted(t *testing.T) {
	cases := []struct {
		name     string
		position int64
		duration int64
		want     bool
	}{
		{name: "no duration", position: 9000, duration: 0, want: false},
		{name: "midway", position: 8000, duration: 20000, want: false},
		{name: "last two seconds", position: 13000, duration: 15000, want: true},
		{name: "ninety five percent", position: 9500, duration: 10000, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsCompleted(tc.position, tc.duration); got != tc.want {
				t.Fatalf("IsCompleted(%d,%d)=%v, want %v", tc.position, tc.duration, got, tc.want)
			}
		})
	}
}

func TestShouldResume(t *testing.T) {
	if ShouldResume(8000, 20000, false) != true {
		t.Fatal("midway unfinished should resume")
	}
	if ShouldResume(8000, 20000, true) {
		t.Fatal("completed must not resume")
	}
	if ShouldResume(1500, 20000, false) {
		t.Fatal("under 2s must not resume")
	}
	if ShouldResume(3000, 60000, false) {
		t.Fatal("under 10 percent must not resume")
	}
	if ShouldResume(14500, 15000, false) {
		t.Fatal("near end must not resume")
	}
	if ResumeMs(8000, 20000, false) != 8000 {
		t.Fatal("resume ms should keep position")
	}
	if ResumeMs(14500, 15000, false) != 0 {
		t.Fatal("near end resume ms should be 0")
	}
}
