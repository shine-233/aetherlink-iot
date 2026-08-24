// 文件用途：维护 plugin\aetherlink\db.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：GetDeviceByVoucher 为凭证哈希双模式匹配入口（hash 优先、明文兜底，
// 见 lookupDeviceByVoucherFromDB）；voucherCacheKey 与 backend/pkg/utils/vouchercache.go
// 是跨服务契约（存储哈希=缓存键算法），任一侧变更需双端同步并更新契约测试。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"crypto/hmac"
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
	VoucherHash     *string    `gorm:"column:voucher_hash" json:"voucher_hash"`
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
// voucherCacheKey 与 backend/pkg/utils/vouchercache.go 的 VoucherCacheKey 保持一致：
// 使用 HMAC-SHA256（域分离密钥 aetherlink:voucher-cache:v1），满足 CodeQL 键控哈希要求，
// 同时保持双端确定性——同一 voucher 在 backend 和 broker 产生相同摘要。
func voucherCacheKey(voucher string) string {
	mac := hmac.New(sha256.New, []byte("aetherlink:voucher-cache:v1"))
	mac.Write([]byte(voucher))
	return hex.EncodeToString(mac.Sum(nil))
}

func GetDeviceByVoucher(voucher string) (*Device, error) {
	var device Device

	deviceID, _ := GetStr(voucherCacheKey(voucher))
	if deviceID == "" {
		Log.Debug("device voucher cache miss")
		dev, err := lookupDeviceByVoucherFromDB(voucher)
		if err != nil {
			Log.Info("load device by voucher failed", zap.Error(err))
			return nil, err
		}
		device = *dev

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

// lookupDeviceByVoucherFromDB 在数据库中按双模式解析设备凭证（凭证哈希存储 Phase 1，
// 见 references/backend-hardening-plan.md 车道1）：先对 deviceVoucherLookupCandidates
// 的每个候选查 voucher_hash=sha256hex(candidate)（走 idx_devices_voucher_hash 索引），
// 全部未命中再回落现有 voucher=? 明文匹配（兼容尚未回填的存量行）。缓存键不受影响，
// 仍取原始 presented voucher 的 sha256（见 GetDeviceByVoucher）。
// Phase 2 停写明文并观测归零后，明文兜底分支随列删除一并移除。
func lookupDeviceByVoucherFromDB(voucher string) (*Device, error) {
	candidates := deviceVoucherLookupCandidates(voucher)

	var device Device
	for _, candidate := range candidates {
		result := db.Model(&Device{}).
			Where("voucher_hash = ?", voucherCacheKey(candidate)).
			First(&device)
		if result.Error == nil {
			return &device, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	var lookupErr error
	for _, candidate := range candidates {
		result := db.Model(&Device{}).Where("voucher = ?", candidate).First(&device)
		if result.Error == nil {
			return &device, nil
		}
		lookupErr = result.Error
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	// candidates 恒非空（至少含原始 voucher），故 lookupErr 必为最后一条 NotFound。
	return nil, lookupErr
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
