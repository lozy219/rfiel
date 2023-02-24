package tiles

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/processing"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/simplify"
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

	collections := map[string]*geojson.FeatureCollection{}
	points, ok := processing.GetMultiPoint(sessionId, x, y, z)
	if !ok {
		c.Data(http.StatusBadRequest, "application/vnd.mapbox-vector-tile", nil)
		return
	}

	fcg, fcy, fco, fcr := geojson.NewFeatureCollection(), geojson.NewFeatureCollection(), geojson.NewFeatureCollection(), geojson.NewFeatureCollection()
	fcg.Append(geojson.NewFeature(points.Green))
	fcy.Append(geojson.NewFeature(points.Yellow))
	fco.Append(geojson.NewFeature(points.Orange))
	fcr.Append(geojson.NewFeature(points.Red))
	collections["green"] = fcg
	collections["yellow"] = fcy
	collections["orange"] = fco
	collections["red"] = fcr

	layers := mvt.NewLayers(collections)
	layers.ProjectToTile(maptile.New(uint32(x), uint32(y), maptile.Zoom(z)))
	layers.Clip(mvt.MapboxGLDefaultExtentBound)
	layers.Simplify(simplify.DouglasPeucker(1.0))

	data, _ := mvt.MarshalGzipped(layers)
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
