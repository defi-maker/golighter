package shared

import (
    "fmt"
    "os"
    "strconv"
    "strings"
)

const defaultEndpoint = "https://mainnet.zklighter.elliot.ai"

func Endpoint() string {
    endpoint := strings.TrimSpace(os.Getenv("LIGHTER_ENDPOINT"))
    if endpoint == "" {
        return defaultEndpoint
    }
    return endpoint
}

func RequireString(key string) (string, error) {
    value := strings.TrimSpace(os.Getenv(key))
    if value == "" {
        return "", fmt.Errorf("missing %s", key)
    }
    return value, nil
}

func RequireInt64(key string) (int64, error) {
    value, err := RequireString(key)
    if err != nil {
        return 0, err
    }
    parsed, parseErr := strconv.ParseInt(value, 10, 64)
    if parseErr != nil {
        return 0, fmt.Errorf("invalid %s: %w", key, parseErr)
    }
    return parsed, nil
}

func RequireUint8(key string) (uint8, error) {
    value, err := RequireString(key)
    if err != nil {
        return 0, err
    }
    parsed, parseErr := strconv.ParseUint(value, 10, 8)
    if parseErr != nil {
        return 0, fmt.Errorf("invalid %s: %w", key, parseErr)
    }
    return uint8(parsed), nil
}

func Uint32OrDefault(key string, fallback uint32) (uint32, error) {
    raw := strings.TrimSpace(os.Getenv(key))
    if raw == "" {
        return fallback, nil
    }
    parsed, err := strconv.ParseUint(raw, 10, 32)
    if err != nil {
        return 0, fmt.Errorf("invalid %s: %w", key, err)
    }
    return uint32(parsed), nil
}
