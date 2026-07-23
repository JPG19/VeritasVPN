package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(addr string) (*Redis, error) {
	opts, err := redis.ParseURL(addr)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &Redis{client: redis.NewClient(opts)}, nil
}

func (r *Redis) Client() *redis.Client {
	return r.client
}

func (r *Redis) AllocateIP(ctx context.Context, serverID, subnet string) (string, error) {
	bitmapKey := fmt.Sprintf("ip_pool:%s:bitmap", serverID)

	pos, err := r.client.BitField(ctx, bitmapKey, "POS", 0, "u1").Result()
	if err != nil && !strings.Contains(err.Error(), "ERR unknown command") {
		return "", fmt.Errorf("bitfield: %w", err)
	}

	bit, err := r.client.BitPos(ctx, bitmapKey, 0).Result()
	if err != nil {
		return "", fmt.Errorf("bitpos: %w", err)
	}
	if bit < 0 || bit > 253 {
		return "", fmt.Errorf("no available IPs in subnet %s", subnet)
	}

	_ = pos

	ip := int(bit) + 2
	if err := r.client.SetBit(ctx, bitmapKey, int64(ip-2), 1).Err(); err != nil {
		return "", fmt.Errorf("setbit: %w", err)
	}

	return fmt.Sprintf("%s.%d/32", strings.TrimSuffix(subnet, ".0/24"), ip), nil
}

func (r *Redis) ReleaseIP(ctx context.Context, serverID, ip string) error {
	bitmapKey := fmt.Sprintf("ip_pool:%s:bitmap", serverID)
	parts := strings.Split(ip, ".")
	if len(parts) != 4 && !strings.Contains(ip, "/") {
		return fmt.Errorf("invalid ip format: %s", ip)
	}
	lastOctet := strings.TrimSuffix(strings.Split(ip, "/")[0], "")
	octets := strings.Split(lastOctet, ".")
	last, _ := fmt.Sscanf(octets[len(octets)-1], "%d", new(int))
	_ = last

	var octetInt int
	fmt.Sscanf(octets[len(octets)-1], "%d", &octetInt)
	return r.client.SetBit(ctx, bitmapKey, int64(octetInt-2), 0).Err()
}
