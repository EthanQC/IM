package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	// 在线用户Key前缀 (Hash: field=deviceID, value=deviceInfoJSON)
	onlineUserKeyPrefix = "im:online:user:"
	// 网关路由Key前缀 (Hash: field=userID:deviceID, value=serverAddr)
	gatewayRouteKeyPrefix = "im:route:gateway:"
	// 在线用户集合Key（全局）
	onlineUsersSetKey = "im:online:users"
	// 在线用户过期时间
	onlineUserTTL = 5 * time.Minute
	// 路由信息过期时间
	routeTTL = 5 * time.Minute
)

// DeviceInfo 设备信息
type DeviceInfo struct {
	DeviceID    string `json:"device_id"`
	Platform    string `json:"platform"`
	ServerAddr  string `json:"server_addr"`
	ConnectedAt int64  `json:"connected_at"`
	LastPingAt  int64  `json:"last_ping_at"`
}

// Lua脚本：原子性设置在线状态和路由
var setOnlineScript = redis.NewScript(`
local user_key = KEYS[1]
local route_key = KEYS[2]
local users_set_key = KEYS[3]
local user_id = ARGV[1]
local device_id = ARGV[2]
local device_info = ARGV[3]
local server_addr = ARGV[4]
local ttl = tonumber(ARGV[5])

-- 设置设备信息
redis.call('HSET', user_key, device_id, device_info)
redis.call('EXPIRE', user_key, ttl)

-- 设置路由信息
local route_field = user_id .. ':' .. device_id
redis.call('HSET', route_key, route_field, server_addr)
redis.call('EXPIRE', route_key, ttl)

-- 添加到在线用户集合
redis.call('SADD', users_set_key, user_id)

return 1
`)

// Lua脚本：原子性设置离线状态和清理路由
var setOfflineScript = redis.NewScript(`
local user_key = KEYS[1]
local route_key = KEYS[2]
local users_set_key = KEYS[3]
local user_id = ARGV[1]
local device_id = ARGV[2]

-- 删除设备信息
redis.call('HDEL', user_key, device_id)

-- 删除路由信息
local route_field = user_id .. ':' .. device_id
redis.call('HDEL', route_key, route_field)

-- 检查用户是否还有其他在线设备
local remaining = redis.call('HLEN', user_key)
if remaining == 0 then
    redis.call('SREM', users_set_key, user_id)
end

return remaining
`)

// EnhancedOnlineUserRepositoryRedis 增强版在线用户仓储实现
type EnhancedOnlineUserRepositoryRedis struct {
	client     *redis.Client
	serverAddr string // 当前服务器地址
}

func NewEnhancedOnlineUserRepositoryRedis(client *redis.Client, serverAddr string) out.OnlineUserRepository {
	return &EnhancedOnlineUserRepositoryRedis{
		client:     client,
		serverAddr: serverAddr,
	}
}

func (r *EnhancedOnlineUserRepositoryRedis) getUserKey(userID uint64) string {
	return fmt.Sprintf("%s%d", onlineUserKeyPrefix, userID)
}

func (r *EnhancedOnlineUserRepositoryRedis) getRouteKey() string {
	return gatewayRouteKeyPrefix + "all"
}

// SetOnline 设置用户在线
func (r *EnhancedOnlineUserRepositoryRedis) SetOnline(ctx context.Context, user *entity.OnlineUser) error {
	userKey := r.getUserKey(user.UserID)
	routeKey := r.getRouteKey()

	deviceInfo := DeviceInfo{
		DeviceID:    user.DeviceID,
		Platform:    user.Platform,
		ServerAddr:  user.ServerAddr,
		ConnectedAt: user.ConnectedAt.Unix(),
		LastPingAt:  time.Now().Unix(),
	}

	deviceInfoJSON, err := json.Marshal(deviceInfo)
	if err != nil {
		return fmt.Errorf("marshal device info failed: %w", err)
	}

	_, err = setOnlineScript.Run(ctx, r.client,
		[]string{userKey, routeKey, onlineUsersSetKey},
		user.UserID, user.DeviceID, string(deviceInfoJSON), user.ServerAddr, int(onlineUserTTL.Seconds()),
	).Result()
	if err != nil {
		return fmt.Errorf("set online failed: %w", err)
	}

	return nil
}

// SetOffline 设置用户离线
func (r *EnhancedOnlineUserRepositoryRedis) SetOffline(ctx context.Context, userID uint64, deviceID string) error {
	userKey := r.getUserKey(userID)
	routeKey := r.getRouteKey()

	_, err := setOfflineScript.Run(ctx, r.client,
		[]string{userKey, routeKey, onlineUsersSetKey},
		userID, deviceID,
	).Result()
	if err != nil {
		return fmt.Errorf("set offline failed: %w", err)
	}

	return nil
}

// GetOnlineDevices 获取用户的所有在线设备
func (r *EnhancedOnlineUserRepositoryRedis) GetOnlineDevices(ctx context.Context, userID uint64) ([]*entity.OnlineUser, error) {
	key := r.getUserKey(userID)
	
	result, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("get online devices failed: %w", err)
	}

	devices := make([]*entity.OnlineUser, 0, len(result))
	for _, data := range result {
		var deviceInfo DeviceInfo
		if err := json.Unmarshal([]byte(data), &deviceInfo); err != nil {
			continue
		}
		
		devices = append(devices, &entity.OnlineUser{
			UserID:      userID,
			DeviceID:    deviceInfo.DeviceID,
			Platform:    deviceInfo.Platform,
			ServerAddr:  deviceInfo.ServerAddr,
			ConnectedAt: time.Unix(deviceInfo.ConnectedAt, 0),
			LastPingAt:  time.Unix(deviceInfo.LastPingAt, 0),
		})
	}

	return devices, nil
}

// IsOnline 检查用户是否在线
func (r *EnhancedOnlineUserRepositoryRedis) IsOnline(ctx context.Context, userID uint64) (bool, error) {
	key := r.getUserKey(userID)
	
	count, err := r.client.HLen(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check online status failed: %w", err)
	}

	return count > 0, nil
}

// GetOnlineUsers 批量获取在线用户
func (r *EnhancedOnlineUserRepositoryRedis) GetOnlineUsers(ctx context.Context, userIDs []uint64) (map[uint64][]*entity.OnlineUser, error) {
	if len(userIDs) == 0 {
		return make(map[uint64][]*entity.OnlineUser), nil
	}

	pipe := r.client.Pipeline()
	cmds := make(map[uint64]*redis.MapStringStringCmd)

	for _, userID := range userIDs {
		key := r.getUserKey(userID)
		cmds[userID] = pipe.HGetAll(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("batch get online users failed: %w", err)
	}

	result := make(map[uint64][]*entity.OnlineUser)
	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil || len(data) == 0 {
			continue
		}

		devices := make([]*entity.OnlineUser, 0, len(data))
		for _, d := range data {
			var deviceInfo DeviceInfo
			if err := json.Unmarshal([]byte(d), &deviceInfo); err != nil {
				continue
			}

			devices = append(devices, &entity.OnlineUser{
				UserID:      userID,
				DeviceID:    deviceInfo.DeviceID,
				Platform:    deviceInfo.Platform,
				ServerAddr:  deviceInfo.ServerAddr,
				ConnectedAt: time.Unix(deviceInfo.ConnectedAt, 0),
				LastPingAt:  time.Unix(deviceInfo.LastPingAt, 0),
			})
		}

		if len(devices) > 0 {
			result[userID] = devices
		}
	}

	return result, nil
}

// UpdateLastPing 更新最后心跳时间
func (r *EnhancedOnlineUserRepositoryRedis) UpdateLastPing(ctx context.Context, userID uint64, deviceID string) error {
	key := r.getUserKey(userID)

	// 获取当前设备信息
	data, err := r.client.HGet(ctx, key, deviceID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("get device info failed: %w", err)
	}

	var deviceInfo DeviceInfo
	if err := json.Unmarshal([]byte(data), &deviceInfo); err != nil {
		return fmt.Errorf("unmarshal device info failed: %w", err)
	}

	deviceInfo.LastPingAt = time.Now().Unix()
	newData, err := json.Marshal(deviceInfo)
	if err != nil {
		return fmt.Errorf("marshal device info failed: %w", err)
	}

	// 更新设备信息并刷新过期时间
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, deviceID, string(newData))
	pipe.Expire(ctx, key, onlineUserTTL)
	_, err = pipe.Exec(ctx)
	
	return err
}

// GetServerRoute 获取用户设备的服务器路由
func (r *EnhancedOnlineUserRepositoryRedis) GetServerRoute(ctx context.Context, userID uint64, deviceID string) (string, error) {
	routeKey := r.getRouteKey()
	routeField := fmt.Sprintf("%d:%s", userID, deviceID)

	serverAddr, err := r.client.HGet(ctx, routeKey, routeField).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("get server route failed: %w", err)
	}

	return serverAddr, nil
}

// GetAllUserRoutes 获取用户所有设备的服务器路由
func (r *EnhancedOnlineUserRepositoryRedis) GetAllUserRoutes(ctx context.Context, userID uint64) (map[string]string, error) {
	devices, err := r.GetOnlineDevices(ctx, userID)
	if err != nil {
		return nil, err
	}

	routes := make(map[string]string)
	for _, device := range devices {
		routes[device.DeviceID] = device.ServerAddr
	}

	return routes, nil
}

// GetOnlineUserCount 获取在线用户总数
func (r *EnhancedOnlineUserRepositoryRedis) GetOnlineUserCount(ctx context.Context) (int64, error) {
	count, err := r.client.SCard(ctx, onlineUsersSetKey).Result()
	if err != nil {
		return 0, fmt.Errorf("get online user count failed: %w", err)
	}
	return count, nil
}

// GetAllOnlineUserIDs 获取所有在线用户ID
func (r *EnhancedOnlineUserRepositoryRedis) GetAllOnlineUserIDs(ctx context.Context) ([]uint64, error) {
	members, err := r.client.SMembers(ctx, onlineUsersSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("get all online user ids failed: %w", err)
	}

	userIDs := make([]uint64, 0, len(members))
	for _, m := range members {
		var userID uint64
		if _, err := fmt.Sscanf(m, "%d", &userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}

	return userIDs, nil
}

// IsLocalConnection 检查连接是否在本地服务器
func (r *EnhancedOnlineUserRepositoryRedis) IsLocalConnection(ctx context.Context, userID uint64, deviceID string) (bool, error) {
	serverAddr, err := r.GetServerRoute(ctx, userID, deviceID)
	if err != nil {
		return false, err
	}
	return serverAddr == r.serverAddr, nil
}

// GetRemoteConnections 获取用户的远程连接（在其他服务器上）
func (r *EnhancedOnlineUserRepositoryRedis) GetRemoteConnections(ctx context.Context, userID uint64) ([]*entity.OnlineUser, error) {
	devices, err := r.GetOnlineDevices(ctx, userID)
	if err != nil {
		return nil, err
	}

	remoteDevices := make([]*entity.OnlineUser, 0)
	for _, device := range devices {
		if device.ServerAddr != r.serverAddr {
			remoteDevices = append(remoteDevices, device)
		}
	}

	return remoteDevices, nil
}
