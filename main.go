package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func newRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Use service name in docker-compose
	})
}

// Insert mock data
func insertMockData(rdb *redis.Client, count int) {
	start := time.Now()
	pipe := rdb.Pipeline()

	for i := 0; i < count; i++ {
		key := "user:form:" + strconv.Itoa(i) + ":question:" + strconv.Itoa(i)
		pipe.Set(ctx, key, "mock_value", 0)
		pipe.SAdd(ctx, "cache:form:keys", key) // Track keys
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Fatal("Failed to insert mock data:", err)
	}
	fmt.Println("Inserted", count, "keys in", time.Since(start))
}

// SCAN + DEL (Slow)
func slowDelete(rdb *redis.Client, pattern string) {
	start := time.Now()
	iter := rdb.Scan(ctx, 0, pattern, 1000).Iterator()
	for iter.Next(ctx) {
		rdb.Del(ctx, iter.Val())
	}
	fmt.Println("SCAN + DEL took:", time.Since(start))
}

// Set-based Bulk Deletion (Faster)
func fastDelete(rdb *redis.Client) {
	start := time.Now()
	keys, _ := rdb.SMembers(ctx, "cache:form:keys").Result()
	if len(keys) > 0 {
		rdb.Del(ctx, keys...)           // Bulk delete
		rdb.Del(ctx, "cache:form:keys") // Clear tracking set
	}
	fmt.Println("Set-based delete took:", time.Since(start))
}

// Lua Script for Fastest Deletion
var deleteScript = redis.NewScript(`
    local keys = redis.call("SCAN", 0, "MATCH", ARGV[1], "COUNT", 1000)
    for _, key in ipairs(keys[2]) do
        redis.call("DEL", key)
    end
    return #keys[2]
`)

func luaDelete(rdb *redis.Client) {
	start := time.Now()
	_, _ = deleteScript.Run(ctx, rdb, nil, "user:form:*").Result()
	fmt.Println("Lua delete took:", time.Since(start))
}

func main() {
	fmt.Println("Redis Key Deletion Techniques")
	rdb := newRedisClient()
	defer rdb.Close()

	insertMockData(rdb, 1_000_000) // Insert 1M keys
	slowDelete(rdb, "user:form:*") // SCAN + DEL
	fmt.Println("====================")
	time.Sleep(60 * time.Minute) // Wait for keys to expire

	insertMockData(rdb, 1_000_000) // Insert 1M keys
	fastDelete(rdb)                // Set Tracking
	fmt.Println("====================")
	time.Sleep(60 * time.Minute) // Wait for keys to expire

	insertMockData(rdb, 1_000_000) // Insert 1M keys
	luaDelete(rdb)                 // Lua Script
	fmt.Println("====================")
	time.Sleep(60 * time.Minute) // Wait for keys to expire

}
