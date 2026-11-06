package tiles

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/processing"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

type inflightSession struct {
	done      chan struct{}
	sessionID int64
}

var (
	nextSessionID     atomic.Int64
	sessionRangeMutex sync.Mutex
	sessionRangeMap   = map[[2]int64]int64{}
	sessionInflight   = map[[2]int64]*inflightSession{}

	emptyGzippedMVT []byte

	gzipBufPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}
	gzipWriterPool = sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
			return w
		},
	}
)

func init() {
	fc := geojson.NewFeatureCollection()
	layers := mvt.Layers{mvt.NewLayer("places", fc)}
	emptyGzippedMVT, _ = mvt.MarshalGzipped(layers)
}

func marshalGzippedPooled(layers mvt.Layers) ([]byte, error) {
	raw, err := mvt.Marshal(layers)
	if err != nil {
		return nil, err
	}
	buf := gzipBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	zw := gzipWriterPool.Get().(*gzip.Writer)
	zw.Reset(buf)

	_, err = zw.Write(raw)
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	var out []byte
	if err == nil {
		out = make([]byte, buf.Len())
		copy(out, buf.Bytes())
	}

	gzipWriterPool.Put(zw)
	gzipBufPool.Put(buf)
	return out, err
}

func sendMVT(c *gin.Context, data []byte) {
	c.Header("Content-Encoding", "gzip")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Data(http.StatusOK, "application/vnd.mapbox-vector-tile", data)
}

func NewSession(c *gin.Context) {
	t1, err1 := strconv.ParseInt(c.Param("t1"), 10, 64)
	t2, err2 := strconv.ParseInt(c.Param("t2"), 10, 64)
	if err1 != nil || err2 != nil || t1 > t2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
		return
	}

	key := [2]int64{t1, t2}
	sessionRangeMutex.Lock()
	if existingID, ok := sessionRangeMap[key]; ok {
		if processing.RefreshSession(existingID) {
			sessionRangeMutex.Unlock()
			c.JSON(http.StatusOK, gin.H{"session_id": existingID, "cached": true})
			return
		}
		delete(sessionRangeMap, key)
	}
	if inf, ok := sessionInflight[key]; ok {
		sessionRangeMutex.Unlock()
		<-inf.done
		c.JSON(http.StatusOK, gin.H{"session_id": inf.sessionID, "cached": true})
		return
	}

	inf := &inflightSession{done: make(chan struct{})}
	sessionInflight[key] = inf
	sessionRangeMutex.Unlock()

	// Sequential positive ID stays well within JS Number.MAX_SAFE_INTEGER (2^53 - 1).
	sessionID := nextSessionID.Add(1)
	processing.LoadSession(t1, t2, sessionID, false)

	sessionRangeMutex.Lock()
	sessionRangeMap[key] = sessionID
	delete(sessionInflight, key)
	inf.sessionID = sessionID
	close(inf.done)
	sessionRangeMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"session_id": sessionID, "cached": false})
}

func process(c *gin.Context, sessionId int64) {
	x, errX := strconv.Atoi(c.Param("x"))
	y, errY := strconv.Atoi(c.Param("y"))
	z, errZ := strconv.Atoi(c.Param("z"))
	if errX != nil || errY != nil || errZ != nil || z < 0 || x < 0 || y < 0 {
		c.Data(http.StatusBadRequest, "application/vnd.mapbox-vector-tile", nil)
		return
	}

	session, ok := processing.GetSession(sessionId)
	if !ok {
		c.Data(http.StatusBadRequest, "application/vnd.mapbox-vector-tile", nil)
		return
	}

	rp := session.GetTile(x, y, z)
	if rp == nil {
		sendMVT(c, emptyGzippedMVT)
		return
	}

	key := processing.TileKey{Z: z, X: x, Y: y}
	if cached, ok := session.GetCachedMVT(key); ok {
		sendMVT(c, cached)
		return
	}

	fc := geojson.NewFeatureCollection()
	for i, mp := range rp.Tiers {
		if len(mp) > 0 {
			// Copy slice before ProjectToTile/Clip mutates coordinates in place.
			mpCopy := make(orb.MultiPoint, len(mp))
			copy(mpCopy, mp)
			feat := geojson.NewFeature(mpCopy)
			feat.Properties["tier"] = i + 1
			fc.Append(feat)
		}
	}

	layers := mvt.Layers{mvt.NewLayer("places", fc)}
	layers.ProjectToTile(maptile.New(uint32(x), uint32(y), maptile.Zoom(z)))
	layers.Clip(mvt.MapboxGLDefaultExtentBound)

	data, err := marshalGzippedPooled(layers)
	if err != nil {
		c.Data(http.StatusInternalServerError, "application/vnd.mapbox-vector-tile", nil)
		return
	}

	session.SetCachedMVT(key, data)
	sendMVT(c, data)
}

func Process(c *gin.Context) {
	process(c, processing.MAIN_SESSION_ID)
}

func ProcessSession(c *gin.Context) {
	sessionId, err := strconv.ParseInt(c.Param("s"), 10, 64)
	if err != nil {
		c.Data(http.StatusBadRequest, "application/vnd.mapbox-vector-tile", nil)
		return
	}
	process(c, sessionId)
}
