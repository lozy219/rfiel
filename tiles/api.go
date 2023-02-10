package tiles

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/processing"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/simplify"
)

func Process(c *gin.Context) {
	x, _ := strconv.Atoi(c.Param("x"))
	y, _ := strconv.Atoi(c.Param("y"))
	z, _ := strconv.Atoi(c.Param("z"))

	collections := map[string]*geojson.FeatureCollection{}
	fc := geojson.NewFeatureCollection()
	fc.Append(geojson.NewFeature(processing.GetMultiPoint(x, y, z)))
	collections["green"] = fc

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
