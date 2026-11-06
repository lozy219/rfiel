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

func BenchmarkLoadSession2024(b *testing.B) {
	// 2024-01-01 to 2024-12-31 UTC
	const t1 = 1704067200
	const t2 = 1735689599
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadSession(t1, t2, int64(1000+i), false)
	}
}

func TestInspectData(t *testing.T) {
	pts, ok := GetMultiPoint(MAIN_SESSION_ID, 909, 403, 10)
	if !ok || pts == nil {
		t.Fatalf("expected valid non-empty Zoom 10 (909, 403) tile, got ok=%v pts=%v", ok, pts)
	}
	z15ChildPoints := 0
	for dx := 0; dx <= 1; dx++ {
		for dy := 0; dy <= 1; dy++ {
			if p15, ok15 := GetMultiPoint(MAIN_SESSION_ID, 14553*2+dx, 6457*2+dy, 15); ok15 && p15 != nil {
				z15ChildPoints += p15.TotalPoints()
			}
		}
	}
	s, _ := GetSession(MAIN_SESSION_ID)
	if len(s.Tiles[15]) == 0 || z15ChildPoints == 0 {
		t.Fatalf("expected non-empty Zoom 15 index, got totalTiles=%d childPoints=%d", len(s.Tiles[15]), z15ChildPoints)
	}
	t.Logf("Zoom 10 (909, 403) totalPoints=%d | Zoom 15 totalTiles=%d childPoints=%d | totalDataPoints=%d",
		pts.TotalPoints(), len(s.Tiles[15]), z15ChildPoints, len(data))
}
