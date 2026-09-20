package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLocationJSONOrString(t *testing.T) {
	t.Run("valid JSON with latitude and longitude", func(t *testing.T) {
		raw := `{"latitude": 39.9042, "longitude": 116.4074, "speed": 62.5}`
		lat, lng, speed, addr := parseLocationJSONOrString(raw)
		assert.NotNil(t, lat)
		assert.NotNil(t, lng)
		assert.NotNil(t, speed)
		assert.InDelta(t, 39.9042, *lat, 0.0001)
		assert.InDelta(t, 116.4074, *lng, 0.0001)
		assert.InDelta(t, 62.5, *speed, 0.0001)
		assert.Nil(t, addr)
	})

	t.Run("valid JSON with short lat and lng", func(t *testing.T) {
		raw := `{"lat": 31.2304, "lng": 121.4737}`
		lat, lng, speed, addr := parseLocationJSONOrString(raw)
		assert.NotNil(t, lat)
		assert.NotNil(t, lng)
		assert.InDelta(t, 31.2304, *lat, 0.0001)
		assert.InDelta(t, 121.4737, *lng, 0.0001)
		assert.Nil(t, speed)
		assert.Nil(t, addr)
	})

	t.Run("comma-separated coordinates lng,lat", func(t *testing.T) {
		raw := "116.4074, 39.9042"
		lat, lng, _, _ := parseLocationJSONOrString(raw)
		assert.NotNil(t, lat)
		assert.NotNil(t, lng)
		assert.InDelta(t, 39.9042, *lat, 0.0001)
		assert.InDelta(t, 116.4074, *lng, 0.0001)
	})

	t.Run("comma-separated coordinates lat,lng", func(t *testing.T) {
		raw := "22.5431, 114.0579"
		lat, lng, _, _ := parseLocationJSONOrString(raw)
		assert.NotNil(t, lat)
		assert.NotNil(t, lng)
		assert.InDelta(t, 22.5431, *lat, 0.0001)
		assert.InDelta(t, 114.0579, *lng, 0.0001)
	})

	t.Run("plain text address string", func(t *testing.T) {
		raw := "北京市朝阳区高新科技园区8号楼"
		lat, lng, _, addr := parseLocationJSONOrString(raw)
		assert.Nil(t, lat)
		assert.Nil(t, lng)
		assert.NotNil(t, addr)
		assert.Equal(t, raw, *addr)
	})
}

func TestCoordinateValidation(t *testing.T) {
	assert.True(t, isValidLatitude(39.9))
	assert.True(t, isValidLatitude(-89.9))
	assert.False(t, isValidLatitude(95.0))
	assert.False(t, isValidLatitude(-91.0))

	assert.True(t, isValidLongitude(116.4))
	assert.True(t, isValidLongitude(-179.9))
	assert.False(t, isValidLongitude(185.0))
	assert.False(t, isValidLongitude(-181.0))
}
