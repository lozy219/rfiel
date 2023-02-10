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
	return 1 / math.Pow(2.0, float64(level+4)) * 268435456 / 10240000
}

func snapToGrid(f, size float64) float64 {
	return float64(int(f/size)) * size
}

func GetMultiPoint(x, y, z int) orb.MultiPoint {
	points := orb.MultiPoint{}
	lat1, lon1 := xyzToLatlon(x, y, z)
	lat2, lon2 := xyzToLatlon(x+1, y+1, z)
	if lat1 > lat2 {
		lat1, lat2 = lat2, lat1
	}
	if lon1 > lon2 {
		lon1, lon2 = lon2, lon1
	}

	// This is super inefficient, but who cares.
	for coord := range lcounter[z] {
		if coord[0] > lon1 && coord[0] <= lon2 && coord[1] > lat1 && coord[1] <= lat2 {
			points = append(points, orb.Point{coord[0], coord[1]})
		}
	}

	return points
}
