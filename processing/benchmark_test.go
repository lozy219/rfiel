package processing

import (
	"testing"
)

func BenchmarkGetMultiPointZoom10(b *testing.B) {
	// Zoom 10 tile around Tokyo (dense GPS tracks)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetMultiPoint(MAIN_SESSION_ID, 909, 403, 10)
	}
}

func BenchmarkGetMultiPointZoom12(b *testing.B) {
	// Zoom 12 tile around Tokyo
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetMultiPoint(MAIN_SESSION_ID, 3638, 1614, 12)
	}
}

func BenchmarkGetMultiPointZoom14(b *testing.B) {
	// Zoom 14 tile around Tokyo
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetMultiPoint(MAIN_SESSION_ID, 14553, 6457, 14)
	}
}

func TestInspectData(t *testing.T) {
	pts, ok := GetMultiPoint(MAIN_SESSION_ID, 909, 403, 10)
	t.Logf("Zoom 10 (909, 403) ok=%v totalPoints=%d totalDataPoints=%d", ok, len(pts.Points), len(data))
	s := sessions[MAIN_SESSION_ID]
	t.Logf("LevelCounter[10] size: %d, LevelCounter[14] size: %d", len(s.LevelCounter[10]), len(s.LevelCounter[14]))
}
