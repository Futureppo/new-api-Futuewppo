package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	ChannelRPMPrefix = "channel:rpm:"
	ChannelTPMPrefix = "channel:tpm:"
	ChannelRPDPrefix = "channel:rpd:"
)

// Memory storage
var (
	memoryMutex    sync.RWMutex
	memoryRPMStore = make(map[string][]int64)
	memoryTPMStore = make(map[string]*MemoryCountItem)
	memoryRPDStore = make(map[string]*MemoryCountItem)
)

type MemoryCountItem struct {
	Count      int64
	Expiration int64 // Unix timestamp
}

func CheckChannelRateLimit(channelId int, modelName string, rpm, tpm, rpd int) error {
	if common.RedisEnabled {
		return checkChannelRateLimitRedis(channelId, modelName, rpm, tpm, rpd)
	}
	return checkChannelRateLimitMemory(channelId, modelName, rpm, tpm, rpd)
}

func RecordChannelRateLimit(channelId int, modelName string, rpm, tpm, rpd int, tokens int) {
	if common.RedisEnabled {
		recordChannelRateLimitRedis(channelId, modelName, rpm, tpm, rpd, tokens)
		return
	}
	recordChannelRateLimitMemory(channelId, modelName, rpm, tpm, rpd, tokens)
}

func GetChannelRateLimitUsage(channelId int, modelName string) (rpm, tpm, rpd int64) {
	if common.RedisEnabled {
		return getChannelRateLimitUsageRedis(channelId, modelName)
	}
	return getChannelRateLimitUsageMemory(channelId, modelName)
}

// Redis Implementation

func checkChannelRateLimitRedis(channelId int, modelName string, rpm, tpm, rpd int) error {
	ctx := context.Background()
	rdb := common.RDB

	// RPM Check (Sliding Window using List)
	if rpm > 0 {
		key := fmt.Sprintf("%s%d:%s", ChannelRPMPrefix, channelId, modelName)
		lenVal, err := rdb.LLen(ctx, key).Result()
		if err == nil && int(lenVal) >= rpm {
			oldTimeVal, err := rdb.LIndex(ctx, key, -1).Int64()
			if err == nil {
				now := time.Now().Unix()
				if now-oldTimeVal < 60 {
					return fmt.Errorf("当前渠道模型负载已饱和")
				}
			}
		}
	}

	// RPD Check (Fixed Window 24h)
	if rpd > 0 {
		key := fmt.Sprintf("%s%d:%s", ChannelRPDPrefix, channelId, modelName)
		val, err := rdb.Get(ctx, key).Int64()
		if err == nil && val >= int64(rpd) {
			return fmt.Errorf("当前渠道模型负载已饱和")
		}
	}

	// TPM Check (Fixed Window 1m)
	if tpm > 0 {
		key := fmt.Sprintf("%s%d:%s", ChannelTPMPrefix, channelId, modelName)
		val, err := rdb.Get(ctx, key).Int64()
		if err == nil && val >= int64(tpm) {
			return fmt.Errorf("当前渠道模型负载已饱和")
		}
	}
	return nil
}

func recordChannelRateLimitRedis(channelId int, modelName string, rpm, tpm, rpd int, tokens int) {
	ctx := context.Background()
	rdb := common.RDB

	// RPM Record
	// Even if rpm limit is 0 (unlimited), we record up to a default limit for monitoring purposes
	limitRPM := int64(rpm)
	if limitRPM <= 0 {
		limitRPM = 1000 // Default monitoring window size
	}
	keyRPM := fmt.Sprintf("%s%d:%s", ChannelRPMPrefix, channelId, modelName)
	rdb.LPush(ctx, keyRPM, time.Now().Unix())
	rdb.LTrim(ctx, keyRPM, 0, limitRPM-1)
	rdb.Expire(ctx, keyRPM, time.Minute)

	// RPD Record
	keyRPD := fmt.Sprintf("%s%d:%s", ChannelRPDPrefix, channelId, modelName)
	val, _ := rdb.Incr(ctx, keyRPD).Result()
	if val == 1 {
		rdb.Expire(ctx, keyRPD, 24*time.Hour)
	}

	// TPM Record
	if tokens > 0 {
		keyTPM := fmt.Sprintf("%s%d:%s", ChannelTPMPrefix, channelId, modelName)
		val, _ := rdb.IncrBy(ctx, keyTPM, int64(tokens)).Result()
		if val == int64(tokens) {
			rdb.Expire(ctx, keyTPM, time.Minute)
		}
	}
}

func getChannelRateLimitUsageRedis(channelId int, modelName string) (rpm, tpm, rpd int64) {
	ctx := context.Background()
	rdb := common.RDB

	// RPM
	rpmKey := fmt.Sprintf("%s%d:%s", ChannelRPMPrefix, channelId, modelName)
	rpm, _ = rdb.LLen(ctx, rpmKey).Result()

	// TPM
	tpmKey := fmt.Sprintf("%s%d:%s", ChannelTPMPrefix, channelId, modelName)
	tpm, _ = rdb.Get(ctx, tpmKey).Int64()

	// RPD
	rpdKey := fmt.Sprintf("%s%d:%s", ChannelRPDPrefix, channelId, modelName)
	rpd, _ = rdb.Get(ctx, rpdKey).Int64()

	return
}

// Memory Implementation

func checkChannelRateLimitMemory(channelId int, modelName string, rpm, tpm, rpd int) error {
	now := time.Now().Unix()
	key := fmt.Sprintf("%d:%s", channelId, modelName)

	memoryMutex.RLock()
	defer memoryMutex.RUnlock()

	// RPM Check
	if rpm > 0 {
		if timestamps, ok := memoryRPMStore[key]; ok {
			count := 0
			for _, ts := range timestamps {
				if now-ts < 60 {
					count++
				}
			}
			if count >= rpm {
				return fmt.Errorf("当前渠道模型负载已饱和")
			}
		}
	}

	// TPM Check
	if tpm > 0 {
		if item, ok := memoryTPMStore[key]; ok {
			if now < item.Expiration && item.Count >= int64(tpm) {
				return fmt.Errorf("当前渠道模型负载已饱和")
			}
		}
	}

	// RPD Check
	if rpd > 0 {
		if item, ok := memoryRPDStore[key]; ok {
			if now < item.Expiration && item.Count >= int64(rpd) {
				return fmt.Errorf("当前渠道模型负载已饱和")
			}
		}
	}

	return nil
}

func recordChannelRateLimitMemory(channelId int, modelName string, rpm, tpm, rpd int, tokens int) {
	now := time.Now().Unix()
	key := fmt.Sprintf("%d:%s", channelId, modelName)

	memoryMutex.Lock()
	defer memoryMutex.Unlock()

	// RPM Record
	timestamps := memoryRPMStore[key]
	// Cleanup expired
	newTimestamps := make([]int64, 0, len(timestamps)+1)
	for _, ts := range timestamps {
		if now-ts < 60 {
			newTimestamps = append(newTimestamps, ts)
		}
	}
	newTimestamps = append(newTimestamps, now)
	// Trim if needed (monitor safety cap)
	limitRPM := rpm
	if limitRPM <= 0 {
		limitRPM = 1000
	}
	if len(newTimestamps) > limitRPM {
		newTimestamps = newTimestamps[len(newTimestamps)-limitRPM:]
	}
	memoryRPMStore[key] = newTimestamps

	// TPM Record
	if tokens > 0 {
		item, ok := memoryTPMStore[key]
		if !ok || now >= item.Expiration {
			item = &MemoryCountItem{Count: int64(tokens), Expiration: now + 60}
		} else {
			item.Count += int64(tokens)
		}
		memoryTPMStore[key] = item
	}

	// RPD Record
	item, ok := memoryRPDStore[key]
	if !ok || now >= item.Expiration {
		item = &MemoryCountItem{Count: 1, Expiration: now + 24*3600}
	} else {
		item.Count++
	}
	memoryRPDStore[key] = item
}

func getChannelRateLimitUsageMemory(channelId int, modelName string) (rpm, tpm, rpd int64) {
	now := time.Now().Unix()
	key := fmt.Sprintf("%d:%s", channelId, modelName)

	memoryMutex.RLock()
	defer memoryMutex.RUnlock()

	// RPM
	if timestamps, ok := memoryRPMStore[key]; ok {
		for _, ts := range timestamps {
			if now-ts < 60 {
				rpm++
			}
		}
	}

	// TPM
	if item, ok := memoryTPMStore[key]; ok {
		if now < item.Expiration {
			tpm = item.Count
		}
	}

	// RPD
	if item, ok := memoryRPDStore[key]; ok {
		if now < item.Expiration {
			rpd = item.Count
		}
	}

	return
}
