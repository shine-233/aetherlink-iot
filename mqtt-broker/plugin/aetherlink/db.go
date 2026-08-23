// 文件用途：维护 plugin\aetherlink\db.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/redis.v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var redisCache *redis.Client
var db *gorm.DB

const defaultCacheTTL = 1 * time.Hour

type Device struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	Name            *string    `gorm:"column:name" json:"name"`
	DeviceType      int16      `gorm:"column:device_type" json:"device_type"`
	Voucher         string     `gorm:"column:voucher" json:"voucher"`
	TenantID        string     `gorm:"column:tenant_id" json:"tenant_id"`
	IsEnabled       string     `gorm:"column:is_enabled" json:"is_enabled"`
	ActivateFlag    string     `gorm:"column:activate_flag" json:"activate_flag"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdateAt        *time.Time `gorm:"column:update_at" json:"update_at"`
	DeviceNumber    string     `gorm:"column:device_number" json:"device_number"`
	ProductID       *string    `gorm:"column:product_id" json:"product_id"`
	ParentID        *string    `gorm:"column:parent_id" json:"parent_id"`
	Protocol        *string    `gorm:"column:protocol" json:"protocol"`
	Label           *string    `gorm:"column:label" json:"label"`
	Location        *string    `gorm:"column:location" json:"location"`
	SubDeviceAddr   *string    `gorm:"column:sub_device_addr" json:"sub_device_addr"`
	CurrentVersion  *string    `gorm:"column:current_version" json:"current_version"`
	AdditionalInfo  *string    `gorm:"column:additional_info" json:"additional_info"`
	ProtocolConfig  *string    `gorm:"column:protocol_config" json:"protocol_config"`
	Remark1         *string    `gorm:"column:remark1" json:"remark1"`
	Remark2         *string    `gorm:"column:remark2" json:"remark2"`
	Remark3         *string    `gorm:"column:remark3" json:"remark3"`
	DeviceConfigID  *string    `gorm:"column:device_config_id" json:"device_config_id"`
	BatchNumber     *string    `gorm:"column:batch_number" json:"batch_number"`
	ActivateAt      *time.Time `gorm:"column:activate_at" json:"activate_at"`
	IsOnline        int16      `gorm:"column:is_online" json:"is_online"`
	AccessWay       *string    `gorm:"column:access_way" json:"access_way"`
	Description     *string    `gorm:"column:description" json:"description"`
	ServiceAccessID *string    `gorm:"column:service_access_id" json:"service_access_id"`
	LastOfflineTime *time.Time `gorm:"column:last_offline_time" json:"last_offline_time"`
}

func (Device) TableName() string {
	return "devices"
}

func createRedisClient() (*redis.Client, error) {
	redisHost := viper.GetString("db.redis.conn")
	dataBase := viper.GetInt("db.redis.db_num")
	password := viper.GetString("db.redis.password")
	log.Println("connecting redis...")
	client := redis.NewClient(&redis.Options{
		Addr:         redisHost,
		Password:     password,
		DB:           dataBase,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		PoolTimeout:  2 * time.Minute,
		IdleTimeout:  10 * time.Minute,
		PoolSize:     1000,
	})

	if _, err := client.Ping().Result(); err != nil {
		log.Println("redis connect failed:", err)
		return nil, fmt.Errorf("connect redis failed: %w", err)
	}

	log.Println("redis connected")
	return client, nil
}

func buildPostgresDSN(user, password, database, host string, port int, sslMode string) string {
	return fmt.Sprintf(
		"user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
		user,
		password,
		database,
		host,
		port,
		sslMode,
	)
}

func postgresLogTarget(user, database, host string, port int) string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s", host, port, database, user)
}

func createPgClient() (*gorm.DB, error) {
	psqladdr := viper.GetString("db.psql.psqladdr")
	psqlport := viper.GetInt("db.psql.psqlport")
	psqluser := viper.GetString("db.psql.psqluser")
	psqlpass := viper.GetString("db.psql.psqlpass")
	psqldb := viper.GetString("db.psql.psqldb")
	sslMode, err := normalizePostgresSSLMode(viper.GetString(postgresSSLModeConfigKey))
	if err != nil {
		return nil, err
	}
	connectionString := buildPostgresDSN(psqluser, psqlpass, psqldb, psqladdr, psqlport, sslMode)

	log.Println("connecting postgres...", postgresLogTarget(psqluser, psqldb, psqladdr, psqlport))
	d, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect postgres failed: %w", err)
	}

	log.Println("postgres connected")
	return d, nil
}

func Init() error {
	rc, err := createRedisClient()
	if err != nil {
		return err
	}
	redisCache = rc

	pg, err := createPgClient()
	if err != nil {
		return err
	}
	db = pg
	return nil
}

func SetStr(key, value string, expiration time.Duration) error {
	return redisCache.Set(key, value, expiration).Err()
}

func GetStr(key string) (string, error) {
	return redisCache.Get(key).Result()
}

func DelKey(key string) error {
	return redisCache.Del(key).Err()
}

func SetNX(key, value string, expiration time.Duration) (bool, error) {
	return redisCache.SetNX(key, value, expiration).Result()
}

func DelNX(key string) error {
	return redisCache.Del(key).Err()
}

func SetRedisForJsondata(key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return redisCache.Set(key, jsonData, expiration).Err()
}

func GetRedisForJsondata(key string, dest interface{}) error {
	val, err := redisCache.Get(key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// voucherCacheKey 返回设备凭证缓存键。缓存键由完整 voucher 的 SHA-256 摘要构成，
// 避免把携带明文 MQTT 口令的 voucher JSON 直接作为 Redis key 落地。
func voucherCacheKey(voucher string) string {
	sum := sha256.Sum256([]byte(voucher))
	return hex.EncodeToString(sum[:])
}

func GetDeviceByVoucher(voucher string) (*Device, error) {
	var device Device

	deviceID, _ := GetStr(voucherCacheKey(voucher))
	if deviceID == "" {
		Log.Debug("device voucher cache miss")
		var lookupErr error
		for _, candidate := range deviceVoucherLookupCandidates(voucher) {
			result := db.Model(&Device{}).Where("voucher = ?", candidate).First(&device)
			if result.Error == nil {
				lookupErr = nil
				break
			}
			lookupErr = result.Error
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				break
			}
		}
		if lookupErr != nil {
			Log.Info("load device by voucher failed", zap.Error(lookupErr))
			return nil, lookupErr
		}

		if err := SetStr(voucherCacheKey(voucher), device.ID, defaultCacheTTL); err != nil {
			return nil, err
		}
		if err := SetRedisForJsondata(device.ID, device, defaultCacheTTL); err != nil {
			return nil, err
		}
	} else {
		d, err := GetDeviceById(deviceID)
		if err != nil {
			return nil, err
		}
		device = *d
	}

	return &device, nil
}

// deviceVoucherLookupCandidates keeps MQTT authentication compatible with
// device vouchers written by both the API and older clients.  The API accepts
// an arbitrary JSON object and Go's encoding/json emits map keys in lexical
// order, while the MQTT broker builds the same username/password object from a
// struct.  The database column is text, so those semantically identical JSON
// objects are not equal as strings.  Try the exact value first, then the two
// stable username/password encodings without weakening credential matching.
func deviceVoucherLookupCandidates(voucher string) []string {
	candidates := []string{voucher}
	var payload mqttVoucherPayload
	if err := json.Unmarshal([]byte(voucher), &payload); err != nil || payload.Username == "" {
		return candidates
	}

	addCandidate := func(candidate string) {
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	if canonical, err := json.Marshal(mqttVoucherPayload{
		Username: payload.Username,
		Password: payload.Password,
	}); err == nil {
		addCandidate(string(canonical))
	}
	if payload.Password != "" {
		if lexical, err := json.Marshal(map[string]string{
			"username": payload.Username,
			"password": payload.Password,
		}); err == nil {
			addCandidate(string(lexical))
		}
	}
	return candidates
}

func GetDeviceById(deviceID string) (*Device, error) {
	var device Device
	result := db.Model(&Device{}).Where("id = ?", deviceID).First(&device)
	if result.Error != nil {
		return nil, result.Error
	}
	if err := SetRedisForJsondata(device.ID, device, defaultCacheTTL); err != nil {
		return nil, err
	}
	return &device, nil
}

func GetDeviceByNumber(deviceNumber string) (*Device, error) {
	var device Device
	result := db.Model(&Device{}).Where("device_number = ?", deviceNumber).First(&device)
	if result.Error != nil {
		return nil, result.Error
	}
	_ = SetRedisForJsondata(device.ID, device, defaultCacheTTL)
	return &device, nil
}

type UserPub struct {
	Attribute string `json:"attribute"`
	Event     string `json:"event"`
}

type UserSub struct {
	Attribute string `json:"attribute"`
	Commands  string `json:"commands"`
}

type UserTopic struct {
	UserPub UserPub `json:"user_pub"`
	UserSub UserSub `json:"user_sub"`
}

// func GetUserTopicByToken(token string) (*UserTopic, error) {
// 	var userTopic UserTopic
// 	device, err := GetDeviceByToken(token)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if device.AdditionalInfo == "" {
// 		return nil, fmt.Errorf("empty")
// 	}
// 	var additionalInfo map[string]interface{}
// 	err = json.Unmarshal([]byte(device.AdditionalInfo), &additionalInfo)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if _, ok := additionalInfo["user_topic"]; !ok {
// 		return nil, fmt.Errorf("empty")
// 	}
// 	userTopicJSON, err := json.Marshal(additionalInfo["user_topic"])
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = json.Unmarshal(userTopicJSON, &userTopic)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &userTopic, nil
// }
