package dca

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	// ParamStorePrefix is the prefix for AWS Systems Manager Parameter Store keys
	ParamStorePrefix = "awsssm"
	// ParamStorePlaintextPrefix is the prefix indicating a plaintext AWS Parameter Store key
	ParamStorePlaintextPrefix = "awsssm://"
	// ParamStoreEncryptedPrefix is the prefix indicating an encrypted AWS Parameter Store key
	ParamStoreEncryptedPrefix = "awsssme://"
)

// HasAWSParamStorePrefix checks if the given string has the AWS Parameter Store prefix
func HasAWSParamStorePrefix(val string) bool {
	return strings.HasPrefix(val, ParamStorePrefix)
}

// HasAWSParamStorePlaintextPrefix checks if the given key has the plaintext AWS Parameter Store prefix
func HasAWSParamStorePlaintextPrefix(key string) bool {
	return strings.HasPrefix(key, ParamStorePlaintextPrefix)
}

// HasAWSParamStoreEncryptedPrefix checks if the given key has the encrypted AWS Parameter Store prefix
func HasAWSParamStoreEncryptedPrefix(key string) bool {
	return strings.HasPrefix(key, ParamStoreEncryptedPrefix)
}

// StripAWSParamStorePrefix removes the AWS Parameter Store prefix from the key if it exists
// Returns the key without the prefix
func StripAWSParamStorePrefix(key string) string {
	if strings.HasPrefix(key, ParamStoreEncryptedPrefix) {
		return strings.TrimPrefix(key, ParamStoreEncryptedPrefix)
	}
	if strings.HasPrefix(key, ParamStorePlaintextPrefix) {
		return strings.TrimPrefix(key, ParamStorePlaintextPrefix)
	}
	return key
}

// GetAWSParamStoreValue retrieves a value, plaintext or encrypted, from AWS Parameter Store based
// on the prefix.
func GetAWSParamStoreValue(ctx context.Context, key string) (_ []byte, err error) {
	defer AddErr(&err, "dca.GetAWSParamStoreValue")
	if HasAWSParamStorePlaintextPrefix(key) {
		return getAWSParamStoreParameter(ctx, key, false)
	} else if HasAWSParamStoreEncryptedPrefix(key) {
		return getAWSParamStoreParameter(ctx, key, true)
	}
	return nil, fmt.Errorf("AWS Param Store key %s has an invalid prefix", key)
}

func getAWSParamStoreParameter(bgCtx context.Context, key string, encrypted bool) (_ []byte, err error) {
	defer WrapErr(&err, "dca.GetAWSParamStoreValue")

	ctx, cancel := context.WithTimeout(bgCtx, time.Second*5)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading AWS configuration: %w", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)

	strippedKey := StripAWSParamStorePrefix(key)

	out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &strippedKey,
		WithDecryption: &encrypted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve parameter from ssm: %v", err)
	}
	return []byte(*out.Parameter.Value), nil
}
