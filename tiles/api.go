package tiles

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/processing"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

var (
	mvtCacheMutex sync.RWMutex
	mvtCache      = map[processing.TileKey][]byte{}
)

func NewSession(c *gin.Context) {
	t1, _ := strconv.Atoi(c.Param("t1"))
	t2, _ := strconv.Atoi(c.Param("t2"))
	sessionId := time.Now().UnixNano()
	processing.LoadSession(int64(t1), int64(t2), sessionId, false)
	c.JSON(http.StatusOK, gin.H{"session_id": sessionId})
}

func process(c *gin.Context, sessionId int64) {
	x, _ := strconv.Atoi(c.Param("x"))
	y, _ := strconv.Atoi(c.Param("y"))
	z, _ := strconv.Atoi(c.Param("z"))

	if sessionId == processing.MAIN_SESSION_ID {
		key := processing.TileKey{Z: z, X: x, Y: y}
		mvtCacheMutex.RLock()
		if cached, ok := mvtCache[key]; ok {
			mvtCacheMutex.RUnlock()
			c.Header("Content-Encoding", `gzip`)
			c.Header("Content-Disposition", `attachment; filename="data.mvt"`)
			c.Header("Access-Control-Allow-Origin", `*`)
			c.Data(http.StatusOK, "application/vnd.mapbox-vector-tile", cached)
			return
		}
		mvtCacheMutex.RUnlock()
	}

	collections := map[string]*geojson.FeatureCollection{}
	points, ok := processing.GetMultiPoint(sessionId, x, y, z)
	if !ok {
		c.Data(http.StatusBadRequest, "application/vnd.mapbox-vector-tile", nil)
		return
	}

	tierMap := map[int]orb.MultiPoint{}
	for _, gp := range points.Points {
		tierMap[gp.Tier] = append(tierMap[gp.Tier], gp.Point)
	}

	fc := geojson.NewFeatureCollection()
	for tier, mp := range tierMap {
		if len(mp) > 0 {
			feat := geojson.NewFeature(mp)
			feat.Properties["tier"] = tier
			fc.Append(feat)
		}
	}
	collections["places"] = fc

	layers := mvt.NewLayers(collections)
	layers.ProjectToTile(maptile.New(uint32(x), uint32(y), maptile.Zoom(z)))
	layers.Clip(mvt.MapboxGLDefaultExtentBound)

	data, _ := mvt.MarshalGzipped(layers)
	if sessionId == processing.MAIN_SESSION_ID {
		key := processing.TileKey{Z: z, X: x, Y: y}
		mvtCacheMutex.Lock()
		mvtCache[key] = data
		mvtCacheMutex.Unlock()
	}
	c.Header("Content-Encoding", `gzip`)
	c.Header("Content-Disposition", `attachment; filename="data.mvt"`)
	c.Header("Access-Control-Allow-Origin", `*`)
	c.Data(http.StatusOK, "application/vnd.mapbox-vector-tile", data)
}

func Process(c *gin.Context) {
	process(c, processing.MAIN_SESSION_ID)
}

func ProcessSession(c *gin.Context) {
	sessionId, _ := strconv.Atoi(c.Param("s"))
	process(c, int64(sessionId))
}
