package cli

import (
	"testing"
)

func TestSupportedRefreshRates(t *testing.T) {
	t.Parallel()
	got := SupportedRefreshRates()
	want := []int{60, 180}
	if len(got) != len(want) {
		t.Fatalf("SupportedRefreshRates() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("SupportedRefreshRates()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestIsSupportedRate(t *testing.T) {
	t.Parallel()
	for _, hz := range []int{60, 180} {
		if !isSupportedRate(hz) {
			t.Errorf("isSupportedRate(%d) = false, want true", hz)
		}
	}
	for _, hz := range []int{0, 30, 90, 120, 144, -60} {
		if isSupportedRate(hz) {
			t.Errorf("isSupportedRate(%d) = true, want false", hz)
		}
	}
}

func TestParseCurrentRefresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want int
	}{
		{
			name: "eDP-1 at 180Hz",
			out:  "Output: 1 eDP-1 abc-123\n\tenabled\n\tconnected\n\tModes:  1:2560x1600@180.00*  2:2560x1600@60.00  3:1920x1200@180.00\n",
			want: 180,
		},
		{
			name: "eDP-1 at 60Hz",
			out:  "Output: 1 eDP-1 abc-123\n\tenabled\n\tconnected\n\tModes:  1:2560x1600@180.00  2:2560x1600@60.00*  3:1920x1200@180.00\n",
			want: 60,
		},
		{
			name: "no current mode marker",
			out:  "Output: 1 eDP-1 abc-123\n\tModes:  1:2560x1600@180.00  2:2560x1600@60.00\n",
			want: 0,
		},
		{
			name: "multiple outputs, picks eDP-1 only",
			out: "Output: 2 HDMI-1 def-456\n\tModes:  1:1920x1080@60.00*  2:1920x1080@144.00\n" +
				"Output: 1 eDP-1 abc-123\n\tModes:  1:2560x1600@180.00*  2:2560x1600@60.00\n",
			want: 180,
		},
		{
			name: "kscreen-doctor missing eDP-1",
			out:  "Output: 2 HDMI-1 def-456\n\tModes:  1:1920x1080@60.00*\n",
			want: 60, // falls back to whole-text scan; finds the only "@60*"
		},
		{
			name: "empty output",
			out:  "",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseCurrentRefresh([]byte(tc.out))
			if got != tc.want {
				t.Errorf("parseCurrentRefresh() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSetRefreshRateUnsupported(t *testing.T) {
	t.Parallel()
	err := SetRefreshRate(144)
	if err == nil {
		t.Fatal("SetRefreshRate(144) = nil, want error")
	}
}
