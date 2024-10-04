package processing

import (
	"math"

	"github.com/paulmach/orb"
)

func xyzToLatlon(x, y, z int) (lat float64, lon float64) {
	n := math.Pi - 2.0*math.Pi*float64(y)/math.Exp2(float64(z))
	lat = 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lon = float64(x)/math.Exp2(float64(z))*360.0 - 180.0
	return lat, lon
}

func gridSizeAtLevel(level int) float64 {
	return 270.0 / math.Pow(2.0, float64(level+7))
}

func snapToGrid(f, size float64) float64 {
	return math.Round(f/size) * size
}

type RenderingPoints struct {
	Green  orb.MultiPoint
	Yellow orb.MultiPoint
	Orange orb.MultiPoint
	Red    orb.MultiPoint
}

func GetMultiPoint(sessionId int64, x, y, z int) (points RenderingPoints, ok bool) {
	var session Session
	if session, ok = sessions[sessionId]; !ok {
		return
	}

	points = RenderingPoints{}
	if z > MAX_LEVEL {
		z = MAX_LEVEL
	}
	lat1, lon1 := xyzToLatlon(x, y, z)
	lat2, lon2 := xyzToLatlon(x+1, y+1, z)
	if lat1 > lat2 {
		lat1, lat2 = lat2, lat1
	}
	if lon1 > lon2 {
		lon1, lon2 = lon2, lon1
	}

	// This is super inefficient, but who cares.
	for coord, dates := range session.LevelCounter[z] {
		if coord[0] > lon1 && coord[0] <= lon2 && coord[1] > lat1 && coord[1] <= lat2 {
			point := orb.Point{coord[0], coord[1]}
			if len(dates) >= session.Threshold[z][0] {
				points.Red = append(points.Red, point)
			} else if len(dates) >= session.Threshold[z][1] {
				points.Orange = append(points.Orange, point)
			} else if len(dates) >= session.Threshold[z][2] {
				points.Yellow = append(points.Yellow, point)
			} else {
				points.Green = append(points.Green, point)
			}
		}
	}

	return
}
